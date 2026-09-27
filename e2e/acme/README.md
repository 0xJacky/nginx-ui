# ACME end-to-end tests

Real HTTP-01 issuance against [Pebble](https://github.com/letsencrypt/pebble),
the test ACME CA that mirrors Let's Encrypt, using the **official** nginx-ui
image (`Dockerfile`, not the demo image) built from the working tree.

The suite lives next to the public demo suite but is separate: its own
Playwright config (`acme/playwright.config.ts`), its own specs
(`acme/tests/`), and a disposable Docker Compose stack.

## What runs

| Service        | Address (`E2E_ACME_SUBNET_PREFIX`.x) | Role |
|----------------|--------------------------------------|------|
| `pebble`       | `.2`  | ACME CA; validates HTTP-01 on port 80 |
| `challtestsrv` | `.3`  | DNS for Pebble; every name resolves to nginx-ui unless a test overrides it |
| `nginx-ui`     | `.10` | system under test, UI on `127.0.0.1:$E2E_ACME_UI_PORT` |
| `backend`      | `.20` | plain nginx: reverse-proxy target and "DNS points elsewhere" host |

`/etc/nginx` and `/etc/nginx-ui` are volumes that start empty (`nocopy`), so
the image seeds its own Nginx config exactly like a fresh Docker install with
empty bind mounts. nginx-ui resolves names through challtestsrv too, so its own
DNS diagnostics see what Pebble sees. The setup project completes the
first-run install with the install secret (read through `docker compose
exec`), a random admin password saved in `$E2E_ACME_DATA_DIR/e2e-admin.json`,
and stores the browser state in `$E2E_ACME_DATA_DIR/e2e-auth-state.json`.
`down.sh` deletes the volumes and that directory.

Specs:

- `tests/api-issuance.spec.ts` creates sites through `POST /api/sites/:name`,
  issues through the `GET /api/domain/:name/cert` websocket and checks the
  result from inside the container with `curl` / `openssl`: plain challenge
  location, challenge location from an `include`, `proxy_pass
  http://localhost:9180`, renewal through an HTTP->HTTPS redirect (and its
  negative twin), DNS pointing at another server (`unauthorized`) or at an
  unused address (`connection`), and issuance started immediately after
  enabling a site.
- `tests/ui-quick-setup.spec.ts` drives Add Site -> Quick Setup (reverse proxy,
  TLS, redirect) -> SSL step -> finished, then checks HTTPS 200 with a Pebble
  certificate, HTTP 301, and that the challenge path is still proxied. All
  wizard selectors are in `pages/siteAdd.ts`.

Every test uses its own random `*.e2e.test` domain, so tests do not depend on
each other. They share one Nginx and therefore run with one worker.

## Run locally

Prerequisites: Docker with linux/amd64 support, Bun, and once
`bun install && bun run install:browser` in `e2e/`.

From the repository root:

```sh
# 1. Build the image (frontend if app/dist is missing, the linux/amd64 binary, docker build)
sh e2e/acme/scripts/build-image.sh

# 2. Start a fresh stack (removes the previous one, its volumes and data dir)
sh e2e/acme/scripts/up.sh

# 3. Run the suite (or one file: ... test:acme -- api-issuance)
bun run --cwd e2e test:acme

# 4. Stop it and delete its volumes and data dir
sh e2e/acme/scripts/down.sh
```

Useful switches:

| Variable | Default | Purpose |
|----------|---------|---------|
| `NGINX_UI_IMAGE` | `nginx-ui-acme-e2e:local` | image to build and run |
| `E2E_ACME_BUILD_FRONTEND=1` | off | rebuild `app/dist` even if it exists (needed after frontend changes) |
| `E2E_ACME_BINARY` | `native` on Linux x86_64 with Go, else `docker` | `native` go build, `docker` via `cloudflare/build-binary.sh`, `skip` reuses `nginx-ui-linux-amd64/nginx-ui` |
| `E2E_ACME_PROJECT` | `nginxui-acme-e2e-suite` | compose project name |
| `E2E_ACME_UI_PORT` | `18181` | host port of the UI |
| `E2E_ACME_CHALLTESTSRV_PORT` | `18056` | host port of the challtestsrv management API |
| `E2E_ACME_SUBNET_PREFIX` | `10.31.0` | first three octets of the /24 network |
| `E2E_ACME_DATA_DIR` | `$RUNNER_TEMP` or `$TMPDIR` + `/<project>` | per-run admin credentials and browser state |
| `E2E_ACME_BASE_URL` | `http://127.0.0.1:$E2E_ACME_UI_PORT` | Playwright base URL |
| `E2E_ACME_KEEP_DATA=1` | off | `down.sh` keeps the volumes and the data dir |

Export the same variables for every script and for the test run; the scripts
(`scripts/env.sh`) and the specs (`lib/env.ts`) share the defaults. Change the
project, ports and subnet together to run a second stack next to this one.

## Debug

- The UI is at `http://127.0.0.1:18181`; log in with the credentials in
  `$E2E_ACME_DATA_DIR/e2e-admin.json`.
- `sh e2e/acme/scripts/logs.sh` writes the compose logs and the bundled Nginx
  logs to `e2e/acme/test-results/`.
- Traces, videos and screenshots of failed tests are in
  `e2e/acme/test-results/`; the HTML report is in `e2e/acme/playwright-report/`
  (`bunx playwright show-report acme/playwright-report` from `e2e/`).
- Look inside the container: `docker compose -p nginxui-acme-e2e-suite -f
  e2e/acme/compose.yml exec nginx-ui sh`.
- Run a failing stack again without restarting it: the setup project reuses
  the credentials in the data dir when nginx-ui is already installed.
- Point a domain elsewhere by hand: `curl -d '{"host":"x.e2e.test.",
  "addresses":["10.31.0.20"]}' http://127.0.0.1:18056/add-a`, undo with
  `/clear-a`.
- Pebble issues from a random root on every start; the specs check the issuer
  name (`Pebble`) and never verify the chain.
- Sites created by the specs stay in the stack until `down.sh`.

## CI

`.github/workflows/e2e-acme.yml` runs nightly, on demand, and on pull
requests labeled `e2e-acme` (again on every push while the label is present).
It builds the binary natively on the runner, uploads the report, traces and
container logs as the `e2e-acme-report` artifact, and always stops the stack.
