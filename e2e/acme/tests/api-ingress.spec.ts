import type { APIRequestContext } from '@playwright/test'
import type { ApiError, Ingress, IngressSpec } from '../lib/ingress'
import { randomBytes } from 'node:crypto'
import { expect, test } from '@playwright/test'
import { clearDomain, pointDomainTo } from '../lib/dns'
import { inContainer, mustInContainer, servedCertificateDNSNames, servedCertificateIssuer, writeContainerFile } from '../lib/docker'
import { addresses } from '../lib/env'
import { answeringBackends, backendPorts, body, condition, createNodeApi, describeStatus, expectStatus, fileExists, httpBody, httpsBody, readFile, sitePaths, startBackends } from '../lib/ingress'
import { uniqueDomain } from '../lib/sites'

// The declarative ingress API (docs/guide/ingress.md) driven like an external
// controller: X-Node-Secret auth, PUT the desired state, poll the status.
// nginx-ui renders the site and upstream files, applies them with one test
// and reload, and issues the certificate from Pebble in the background.

let api: APIRequestContext

test.beforeAll(async () => {
  api = await createNodeApi()
  await startBackends()
})

test.afterAll(async () => {
  await api?.dispose()
})

function uniqueName(label: string) {
  return `${label}-${randomBytes(3).toString('hex')}`
}

function proxySpec(host: string, tls: IngressSpec['tls'] = { mode: 'none' }): IngressSpec {
  return {
    hosts: [host],
    backends: [{
      name: 'app',
      keepalive: 4,
      servers: [
        { address: `127.0.0.1:${backendPorts.one}` },
        { address: `127.0.0.1:${backendPorts.two}` },
      ],
    }],
    routes: [{ path_prefix: '/', backend: 'app' }],
    tls,
  }
}

async function getIngress(name: string): Promise<Ingress> {
  const response = await expectStatus(await api.get(`/api/ingresses/${name}`), 200, `GET /api/ingresses/${name}`)
  return body<Ingress>(response)
}

async function expectError(response: Awaited<ReturnType<APIRequestContext['get']>>, status: number, scope: string, code: number) {
  const text = await response.text()
  expect(response.status(), text).toBe(status)
  const error = JSON.parse(text) as ApiError
  expect(error.scope, text).toBe(scope)
  expect(error.code, text).toBe(code)
}

test('an ACME ingress goes Ready, balances, drains, resists drift and is deleted', async () => {
  test.setTimeout(300_000)
  const name = uniqueName('ig')
  const host = uniqueDomain('ingress')
  const owner = 'e2e-controller'
  const paths = sitePaths(name)
  const spec = proxySpec(host, { mode: 'acme', challenge: 'http01', redirect_https: true })

  await test.step('PUT with ?wait applies the HTTP listener and starts issuance', async () => {
    const response = await api.put(`/api/ingresses/${name}?wait=60s`, { data: { owner, spec } })
    await expectStatus(response, 200, `PUT /api/ingresses/${name}`)
    const ing = await body<Ingress>(response)
    expect(ing.generation).toBe(1)
    expect(ing.status.observed_generation, describeStatus(ing)).toBe(1)
    expect(condition(ing, 'ConfigApplied')?.status, describeStatus(ing)).toBe(true)
    expect(fileExists(paths.available)).toBe(true)
    expect(fileExists(paths.enabled)).toBe(true)
    expect(readFile(paths.upstream('app'))).toContain(`127.0.0.1:${backendPorts.two}`)

    // Repeating the same PUT is idempotent.
    const again = await body<Ingress>(await expectStatus(
      await api.put(`/api/ingresses/${name}?wait=30s`, { data: { owner, spec } }),
      200,
      'repeated PUT',
    ))
    expect(again.generation).toBe(1)
  })

  await test.step('the certificate is issued by Pebble and the ingress becomes Ready', async () => {
    await expect.poll(async () => {
      const ing = await getIngress(name)
      return `${ing.status.phase} ${condition(ing, 'CertReady')?.reason ?? ''}`
    }, { timeout: 150_000, intervals: [1_000] }).toBe('Ready Issued')
    const ing = await getIngress(name)
    expect(condition(ing, 'CertReady')?.status, describeStatus(ing)).toBe(true)
    expect(ing.status.cert_id, describeStatus(ing)).toBeGreaterThan(0)
    expect(ing.status.cert_not_after, describeStatus(ing)).toBeTruthy()
    expect(readFile(paths.available)).toContain('listen 443 ssl')
  })

  await test.step('HTTPS reaches both servers with the forwarded headers', async () => {
    await expect.poll(() => httpsBody(host), { timeout: 15_000 }).toContain('backend=')
    expect(servedCertificateIssuer(host)).toContain('Pebble')
    expect(answeringBackends(host)).toEqual(['one', 'two'])
    const echoed = httpsBody(host)
    expect(echoed).toContain(`host=${host}`)
    expect(echoed).toContain('xff=203.0.113.7, 127.0.0.1')
    expect(echoed).toContain('proto=https')

    // tls.redirect_https: plain HTTP is redirected.
    const http = inContainer(`curl -sS -o /dev/null -w '%{http_code} %{redirect_url}' --resolve '${host}:80:127.0.0.1' 'http://${host}/x'`).stdout
    expect(http).toBe(`301 https://${host}/x`)
  })

  await test.step('adding a host reissues the certificate while the first host stays on HTTPS', async () => {
    const host2 = uniqueDomain('ingress-added')
    spec.hosts = [host, host2]
    const response = await api.put(`/api/ingresses/${name}?wait=30s`, { data: { owner, spec } })
    await expectStatus(response, 200, 'PUT with an added host')
    const ing = await body<Ingress>(response)
    expect(ing.generation).toBe(2)
    expect(ing.status.observed_generation, describeStatus(ing)).toBe(2)
    // The current certificate keeps serving the host it covers.
    expect(condition(ing, 'CertReady')?.status, describeStatus(ing)).toBe(true)
    expect(httpsBody(host)).toContain('backend=')

    await expect.poll(async () => {
      const current = await getIngress(name)
      return `${current.status.phase} ${condition(current, 'CertReady')?.reason ?? ''} ${servedCertificateDNSNames(host2).join(',')}`
    }, { timeout: 120_000, intervals: [1_000] }).toBe(`Ready Issued ${[host, host2].sort().join(',')}`)
    await expect.poll(() => httpsBody(host2), { timeout: 15_000 }).toContain(`host=${host2}`)
  })

  await test.step('PATCH marks a server down and only the other one answers', async () => {
    const response = await api.patch(`/api/ingresses/${name}/backends/app/servers?wait=30s`, {
      data: { owner, address: `127.0.0.1:${backendPorts.two}`, down: true },
    })
    await expectStatus(response, 200, 'PATCH servers')
    const ing = await body<Ingress>(response)
    expect(ing.generation).toBe(3)
    expect(ing.status.observed_generation, describeStatus(ing)).toBe(3)
    expect(ing.status.servers?.find(s => s.address.endsWith(`:${backendPorts.two}`))?.down).toBe(true)
    expect(readFile(paths.upstream('app'))).toMatch(new RegExp(`127\\.0\\.0\\.1:${backendPorts.two}[^;]*down;`))
    await expect.poll(() => answeringBackends(host), { timeout: 15_000 }).toEqual(['one'])

    // A server that is not in the backend.
    await expectError(await api.patch(`/api/ingresses/${name}/backends/app/servers`, {
      data: { owner, address: '127.0.0.1:24999', down: true },
    }), 400, 'ingress', 40009)
  })

  await test.step('a dry run returns the rendered config and changes nothing', async () => {
    const before = await getIngress(name)
    const siteBefore = readFile(paths.available)
    const draft: IngressSpec = { ...before.spec, policy: { extra_headers: [{ name: 'X-E2E-Dry-Run', value: 'yes' }] } }
    const response = await api.put(`/api/ingresses/${name}?dry_run=1`, { data: { owner, spec: draft } })
    await expectStatus(response, 200, 'dry run')
    const result = await body<{ changed: boolean, error?: string, rendered: { site_name: string, site_content: string, upstreams: Record<string, string> } }>(response)
    expect(result.error ?? '', JSON.stringify(result)).toBe('')
    expect(result.changed).toBe(true)
    expect(result.rendered.site_name).toBe(`ingress-${name}`)
    expect(result.rendered.site_content).toContain('X-E2E-Dry-Run')
    expect(Object.keys(result.rendered.upstreams)).toEqual([`ig-${name}-app`])

    expect(readFile(paths.available)).toBe(siteBefore)
    const after = await getIngress(name)
    expect(after.generation).toBe(before.generation)
    expect(after.spec.policy?.extra_headers ?? []).toEqual([])
  })

  await test.step('another owner gets 409 and force transfers the ingress', async () => {
    const current = (await getIngress(name)).spec
    await expectError(await api.put(`/api/ingresses/${name}`, { data: { owner: 'e2e-intruder', spec: current } }), 409, 'ingress', 40901)
    await expectError(await api.patch(`/api/ingresses/${name}/backends/app/servers`, {
      data: { owner: 'e2e-intruder', address: `127.0.0.1:${backendPorts.two}`, down: false },
    }), 409, 'ingress', 40901)
    await expectError(await api.delete(`/api/ingresses/${name}?owner=e2e-intruder`), 409, 'ingress', 40901)

    const taken = await body<Ingress>(await expectStatus(
      await api.put(`/api/ingresses/${name}?wait=30s`, { data: { owner: 'e2e-intruder', force: true, spec: current } }),
      200,
      'forced PUT',
    ))
    expect(taken.owner).toBe('e2e-intruder')
    // An owner change alone is not a spec change.
    expect(taken.generation).toBe(3)

    const back = await body<Ingress>(await expectStatus(
      await api.put(`/api/ingresses/${name}?wait=30s`, { data: { owner, force: true, spec: current } }),
      200,
      'forced PUT back',
    ))
    expect(back.owner).toBe(owner)
  })

  await test.step('a hand-edited site and upstream file are restored by reconcile', async () => {
    const site = readFile(paths.available)
    const upstream = readFile(paths.upstream('app'))
    mustInContainer(`echo '# hand edit' >> ${paths.available}`)
    mustInContainer(`echo '# hand edit' >> ${paths.upstream('app')}`)
    expect(readFile(paths.available)).not.toBe(site)

    await expectStatus(await api.post(`/api/ingresses/${name}/reconcile`), 200, 'POST reconcile')
    await expect.poll(() => readFile(paths.available), { timeout: 20_000 }).toBe(site)
    await expect.poll(() => readFile(paths.upstream('app')), { timeout: 20_000 }).toBe(upstream)
    expect(answeringBackends(host)).toEqual(['one'])
  })

  await test.step('the site API refuses to change the generated site unless forced', async () => {
    const siteName = `ingress-${name}`
    const site = readFile(paths.available)
    const managed = await (await api.get(`/api/sites/${siteName}`)).json() as { managed_by?: string }
    expect(managed.managed_by).toBe(`ingress:${name}`)

    const edited = site.replace('proxy_http_version 1.1;', 'proxy_http_version 1.1;\n        add_header X-E2E-Forced yes;')
    await expectError(await api.post(`/api/sites/${siteName}`, {
      data: { content: edited, overwrite: true, post_action: 'reload_nginx' },
    }), 409, 'site', 40901)
    await expectError(await api.post(`/api/sites/${siteName}/disable`), 409, 'site', 40901)
    expect(readFile(paths.available)).toBe(site)
    expect(fileExists(paths.enabled)).toBe(true)

    await expectStatus(await api.post(`/api/sites/${siteName}`, {
      data: { content: edited, overwrite: true, post_action: 'reload_nginx', force: true },
    }), 200, 'forced site save')
    expect(readFile(paths.available)).toContain('X-E2E-Forced')

    // The next reconcile overwrites the forced change.
    await expectStatus(await api.post(`/api/ingresses/${name}/reconcile`), 200, 'POST reconcile')
    await expect.poll(() => readFile(paths.available), { timeout: 20_000 }).toBe(site)
  })

  await test.step('DELETE removes the files and the HTTPS endpoint', async () => {
    await expectError(await api.delete(`/api/ingresses/${name}`), 400, 'ingress', 40002)
    const response = await api.delete(`/api/ingresses/${name}?owner=${owner}`)
    await expectStatus(response, 200, 'DELETE')
    expect(fileExists(paths.available)).toBe(false)
    expect(fileExists(paths.enabled)).toBe(false)
    expect(fileExists(paths.upstream('app'))).toBe(false)
    await expectError(await api.get(`/api/ingresses/${name}`), 404, 'ingress', 40401)
    await expect.poll(() => httpsBody(host), { timeout: 15_000 }).not.toContain('backend=')
  })
})

test('the reconcile loop restores a hand-edited file on its own', async () => {
  // compose.yml lowers [ingress] ReconcileIntervalMinutes to 1; the first pass
  // runs a minute after start.
  test.setTimeout(240_000)
  const name = uniqueName('drift')
  const host = uniqueDomain('drift')
  const paths = sitePaths(name)
  await expectStatus(await api.put(`/api/ingresses/${name}?wait=30s`, { data: { owner: 'e2e-drift', spec: proxySpec(host) } }), 200, 'PUT')
  const site = readFile(paths.available)

  mustInContainer(`echo '# hand edit' >> ${paths.available}`)
  await expect.poll(() => readFile(paths.available), { timeout: 150_000, intervals: [2_000] }).toBe(site)
  const ing = await getIngress(name)
  expect(condition(ing, 'ConfigApplied')?.status, describeStatus(ing)).toBe(true)

  await expectStatus(await api.delete(`/api/ingresses/${name}?owner=e2e-drift`), 200, 'DELETE')
})

test('a sync with prune deletes only the given owner\'s ingresses', async () => {
  const keep = uniqueName('keep')
  const drop = uniqueName('drop')
  const keepHost = uniqueDomain('keep')
  await expectStatus(await api.put(`/api/ingresses/${keep}?wait=30s`, { data: { owner: 'e2e-owner-b', spec: proxySpec(keepHost) } }), 200, 'PUT keep')
  const sync = await api.put(`/api/ingresses?owner=e2e-owner-x&wait=30s`, {
    data: { items: [{ name: drop, spec: proxySpec(uniqueDomain('drop')) }] },
  })
  await expectStatus(sync, 200, 'sync owner x')
  expect((await body<{ data: Ingress[] }>(sync)).data.map(i => i.name)).toEqual([drop])
  expect(fileExists(sitePaths(drop).available)).toBe(true)

  // Another owner cannot claim x's ingress through a sync.
  await expectError(await api.put(`/api/ingresses?owner=e2e-owner-b`, {
    data: { items: [{ name: drop, spec: proxySpec(uniqueDomain('steal')) }] },
  }), 409, 'ingress', 40901)

  const pruned = await api.put(`/api/ingresses?owner=e2e-owner-x&prune=1&wait=30s`, { data: { items: [] } })
  await expectStatus(pruned, 200, 'prune owner x')
  expect((await body<{ data: Ingress[] }>(pruned)).data).toEqual([])
  await expect.poll(async () => (await api.get(`/api/ingresses/${drop}`)).status(), { timeout: 20_000 }).toBe(404)
  expect(fileExists(sitePaths(drop).available)).toBe(false)

  const kept = await getIngress(keep)
  expect(kept.owner).toBe('e2e-owner-b')
  expect(fileExists(sitePaths(keep).available)).toBe(true)
  const list = await body<{ data: Ingress[] }>(await api.get('/api/ingresses?owner=e2e-owner-b'))
  expect(list.data.map(i => i.name)).toContain(keep)

  await expectStatus(await api.delete(`/api/ingresses/${keep}?owner=e2e-owner-b`), 200, 'DELETE keep')
})

test('a backend outside AllowedBackends is refused', async () => {
  const name = uniqueName('deny')
  const spec = proxySpec(uniqueDomain('deny'))
  spec.backends[0]!.servers = [{ address: `${addresses.backend}:80` }]
  await expectError(await api.put(`/api/ingresses/${name}`, { data: { owner: 'e2e-controller', spec } }), 400, 'ingress', 40005)
  await expectError(await api.get(`/api/ingresses/${name}`), 404, 'ingress', 40401)
  expect(fileExists(sitePaths(name).available)).toBe(false)
})

test('the ingress API rejects requests without credentials and with a wrong node secret', async ({ playwright }) => {
  const anonymous = await playwright.request.newContext({ baseURL: test.info().project.use.baseURL })
  expect((await anonymous.get('/api/ingresses')).status()).toBe(403)
  const wrong = await playwright.request.newContext({
    baseURL: test.info().project.use.baseURL,
    extraHTTPHeaders: { 'X-Node-Secret': 'not-the-secret' },
  })
  expect((await wrong.put('/api/ingresses/nope', { data: { owner: 'x', spec: proxySpec(uniqueDomain('nope')) } })).status()).toBe(403)
  await anonymous.dispose()
  await wrong.dispose()
})

test('an apply that fails nginx -t is rolled back and a repeated PUT retries it', async () => {
  const name = uniqueName('rollback')
  const host = uniqueDomain('rollback')
  const owner = 'e2e-controller'
  const paths = sitePaths(name)
  const spec = proxySpec(host)
  await expectStatus(await api.put(`/api/ingresses/${name}?wait=30s`, { data: { owner, spec } }), 200, 'PUT generation 1')
  const site = readFile(paths.available)

  // A hand-written upstream that takes the name the next generation renders
  // makes the combined nginx test fail.
  const clash = `/etc/nginx/conf.d/e2e-clash-${name}.conf`
  writeContainerFile(clash, `upstream ig-${name}-extra {\n    server 127.0.0.1:1;\n}\n`)
  try {
    spec.backends.push({ name: 'extra', servers: [{ address: `127.0.0.1:${backendPorts.two}` }] })
    spec.routes.push({ path_prefix: '/extra', backend: 'extra' })
    const failed = await body<Ingress>(await expectStatus(
      await api.put(`/api/ingresses/${name}?wait=30s`, { data: { owner, spec } }),
      200,
      'PUT generation 2',
    ))
    expect(failed.generation).toBe(2)
    expect(failed.status.observed_generation, describeStatus(failed)).toBe(2)
    expect(failed.status.phase, describeStatus(failed)).toBe('Error')
    const applied = condition(failed, 'ConfigApplied')
    expect(applied?.status, describeStatus(failed)).toBe(false)
    expect(applied?.reason, describeStatus(failed)).toBe('TestFailed')
    expect(applied?.message, describeStatus(failed)).toContain(`duplicate upstream "ig-${name}-extra"`)

    // Every file of the failed apply was restored; generation 1 still serves.
    expect(readFile(paths.available)).toBe(site)
    expect(fileExists(paths.upstream('extra'))).toBe(false)
    expect(httpBody(host)).toContain('backend=')
  }
  finally {
    mustInContainer(`rm -f ${clash}`)
  }

  // Repeating the PUT re-applies the same generation.
  await expectStatus(await api.put(`/api/ingresses/${name}`, { data: { owner, spec } }), 200, 'repeated PUT')
  await expect.poll(async () => (await getIngress(name)).status.phase, { timeout: 20_000 }).toBe('Ready')
  expect(fileExists(paths.upstream('extra'))).toBe(true)
  await expect.poll(() => httpBody(host, '/extra'), { timeout: 15_000 }).toContain('backend=two')

  await expectStatus(await api.delete(`/api/ingresses/${name}?owner=${owner}`), 200, 'DELETE')
  expect(fileExists(paths.upstream('extra'))).toBe(false)
})

test('hosts and site names that belong to someone else are refused', async () => {
  const owner = 'e2e-controller'
  const first = uniqueName('first')
  const host = uniqueDomain('taken')
  await expectStatus(await api.put(`/api/ingresses/${first}?wait=30s`, { data: { owner, spec: proxySpec(host) } }), 200, 'PUT first')
  await expectError(await api.put(`/api/ingresses/${uniqueName('second')}`, { data: { owner: 'e2e-other', spec: proxySpec(host) } }), 409, 'ingress', 40902)

  // A regular site already holds the generated site name.
  const squatter = uniqueName('squat')
  const squatterHost = uniqueDomain('squat')
  await expectStatus(await api.post(`/api/sites/ingress-${squatter}`, {
    data: { content: `server {\n    listen 80;\n    server_name ${squatterHost};\n    return 204;\n}\n`, overwrite: true },
  }), 200, 'POST regular site')
  await expectError(await api.put(`/api/ingresses/${squatter}`, { data: { owner, spec: proxySpec(squatterHost) } }), 409, 'ingress', 40903)
  expect(readFile(sitePaths(squatter).available)).toContain('return 204;')

  await expectStatus(await api.delete(`/api/ingresses/${first}?owner=${owner}`), 200, 'DELETE first')
})

test('a failed issuance degrades the ingress and the backoff retry recovers it', async () => {
  // The first retry runs a minute after the failure.
  test.setTimeout(240_000)
  const name = uniqueName('retry')
  const host = uniqueDomain('retry')
  const owner = 'e2e-controller'
  await pointDomainTo(host, addresses.backend)
  try {
    await expectStatus(await api.put(`/api/ingresses/${name}?wait=30s`, {
      data: { owner, spec: proxySpec(host, { mode: 'acme', redirect_https: true }) },
    }), 200, 'PUT')
    await expect.poll(async () => {
      const ing = await getIngress(name)
      return `${ing.status.phase} ${condition(ing, 'CertReady')?.reason ?? ''}`
    }, { timeout: 60_000, intervals: [1_000] }).toBe('Degraded IssueFailed')
    const failed = await getIngress(name)
    expect(condition(failed, 'CertReady')?.message, describeStatus(failed)).toContain('unauthorized')
    // Only the HTTP listener is rendered, and it proxies instead of redirecting.
    expect(readFile(sitePaths(name).available)).not.toContain('listen 443')
    expect(httpBody(host)).toContain('backend=')
  }
  finally {
    await clearDomain(host)
  }

  await expect.poll(async () => {
    const ing = await getIngress(name)
    return `${ing.status.phase} ${condition(ing, 'CertReady')?.reason ?? ''}`
  }, { timeout: 150_000, intervals: [2_000] }).toBe('Ready Issued')
  await expect.poll(() => httpsBody(host), { timeout: 15_000 }).toContain('proto=https')

  await expectStatus(await api.delete(`/api/ingresses/${name}?owner=${owner}`), 200, 'DELETE')
})
