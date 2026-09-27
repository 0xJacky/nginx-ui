import type { Locator, Page, WebSocket } from '@playwright/test'
import { expect } from '@playwright/test'

// Every selector of the "Add Site" wizard lives here. The SSL step is about
// to be redesigned; when it changes, update this file and keep the specs.

export interface QuickSetupInput {
  name: string
  domains: string[]
  upstreamHost: string
  upstreamPort: string
  enableTLS: boolean
  redirectHTTPToHTTPS: boolean
}

export interface CertificateOutcome {
  status: 'success' | 'error'
  message: string
  frames: Record<string, unknown>[]
}

function isIssuanceSocket(url: string) {
  const { pathname } = new URL(url)
  // Legacy per-certificate socket and the HTTPS onboarding orchestrator.
  return /^\/api\/domain\/[^/]+\/cert$/.test(pathname) || /^\/api\/sites\/[^/]+\/https$/.test(pathname)
}

function isFinalFrame(frame: Record<string, unknown>) {
  // Orchestrator events carry a type; only "done" ends the run ("step"
  // events can report status "error" before the rollback finishes).
  if (frame.type !== undefined)
    return frame.type === 'done'
  return frame.status === 'error'
    || (frame.status === 'success' && typeof frame.ssl_certificate === 'string')
}

export class SiteAddPage {
  constructor(readonly page: Page) {}

  private formItem(label: string): Locator {
    return this.page.locator('.ant-form-item').filter({ has: this.page.getByText(label, { exact: true }) }).first()
  }

  private async setSwitch(label: string, checked: boolean) {
    const toggle = this.formItem(label).locator('.ant-switch')
    const isChecked = (await toggle.getAttribute('class'))?.includes('ant-switch-checked') ?? false
    if (isChecked !== checked)
      await toggle.click()
  }

  get nextButton() {
    return this.page.getByRole('button', { name: 'Next', exact: true })
  }

  get activeStep() {
    return this.page.locator('.ant-steps-item-active')
  }

  async open() {
    await this.page.goto('/#/sites/add', { waitUntil: 'domcontentloaded' })
    await expect(this.page.getByText('Quick Setup', { exact: true })).toBeVisible()
  }

  /** Fill Quick Setup for a reverse proxy and advance to the DNS step. */
  async quickSetupReverseProxy(input: QuickSetupInput) {
    await this.page.locator('.ant-radio-button-wrapper').filter({ hasText: 'Reverse Proxy' }).click()
    await this.formItem('Domains').locator('input').fill(input.domains.join(' '))
    await this.formItem('Configuration Name').locator('input').fill(input.name)
    await this.formItem('Host').locator('input').fill(input.upstreamHost)
    await this.formItem('Port').locator('input').fill(input.upstreamPort)
    await this.setSwitch('Enable TLS', input.enableTLS)
    if (input.enableTLS)
      await this.setSwitch('Redirect HTTP to HTTPS', input.redirectHTTPToHTTPS)

    const generated = this.page.waitForResponse(response =>
      new URL(response.url()).pathname === '/api/templates/quick_config' && response.request().method() === 'POST')
    await this.nextButton.click()
    expect((await generated).ok()).toBe(true)
    await expect(this.activeStep).toContainText('DNS Record')
  }

  async skipDNSStep() {
    await this.nextButton.click()
    await expect(this.activeStep).toContainText('Configure SSL')
  }

  /**
   * Start Let's Encrypt issuance from the SSL step and wait for the final
   * websocket frame. The outcome comes from the socket, not from the UI text,
   * so it survives a redesign of the progress display. Both the HTTPS card
   * and the previous per-server switch flow are supported while the redesign
   * lands; drop the legacy branch once it is gone.
   */
  async issueCertificate(timeoutMs = 120_000): Promise<CertificateOutcome> {
    const frames: Record<string, unknown>[] = []
    let settle!: (outcome: CertificateOutcome) => void
    const outcome = new Promise<CertificateOutcome>(resolve => {
      settle = resolve
    })
    const onSocket = (ws: WebSocket) => {
      if (!isIssuanceSocket(ws.url()))
        return
      ws.on('framereceived', ({ payload }) => {
        const frame = JSON.parse(String(payload)) as Record<string, unknown>
        frames.push(frame)
        if (isFinalFrame(frame))
          settle({ status: frame.status === 'success' ? 'success' : 'error', message: String(frame.message ?? ''), frames })
      })
      ws.on('close', () => settle({ status: 'error', message: 'socket closed before a final frame', frames }))
    }
    this.page.on('websocket', onSocket)

    const timeout = new Promise<CertificateOutcome>(resolve => setTimeout(
      () => resolve({ status: 'error', message: `no final frame within ${timeoutMs}ms`, frames }),
      timeoutMs,
    ))

    try {
      // Current SSL step: one HTTPS card that runs the backend orchestrator.
      const httpsCardSubmit = this.page.getByRole('button', { name: 'Issue and enable HTTPS', exact: true })
      // Previous SSL step: warning banner + per-server Let's Encrypt switch.
      const legacyEntry = this.page.getByRole('button', { name: 'Issue certificate', exact: true })
      const letsEncrypt = this.formItem('Encrypt website with Let\'s Encrypt').locator('.ant-switch')

      await expect(httpsCardSubmit.or(legacyEntry).or(letsEncrypt).first()).toBeVisible()

      if (await httpsCardSubmit.isVisible()) {
        // Enabled once the wizard has saved the site draft.
        await expect(httpsCardSubmit).toBeEnabled()
        await httpsCardSubmit.click()
        const result = await Promise.race([outcome, timeout])
        if (result.status === 'success')
          await expect(this.page.getByText('HTTPS is enabled', { exact: true }).first()).toBeVisible()
        return result
      }

      // The banner's "Issue certificate" button is the entry point a user
      // follows; it must bring up the TLS server's certificate section.
      if (await legacyEntry.isVisible())
        await legacyEntry.click()

      await expect(letsEncrypt, 'the SSL step should show the Let\'s Encrypt switch of the TLS server').toBeVisible()
      await letsEncrypt.click()

      // "Do you want to enable TLS?" confirmation.
      await this.page.locator('.ant-modal-confirm').getByRole('button', { name: 'OK' }).click()

      // "Obtain certificate" dialog: HTTP-01 is the default challenge.
      const dialog = this.page.locator('.ant-modal').filter({ hasText: 'Obtain certificate' })
      await dialog.getByRole('button', { name: 'Next', exact: true }).click()

      const result = await Promise.race([outcome, timeout])
      if (result.status === 'success') {
        // Let the dialog finish saving the certificate directives, then close it.
        await expect(dialog.locator('.ant-progress-status-success')).toBeVisible()
        await this.page.keyboard.press('Escape')
        await expect(dialog).toBeHidden()
      }
      return result
    }
    finally {
      this.page.off('websocket', onSocket)
    }
  }

  /** Wait for the "created" result page; the HTTPS card finishes the wizard by itself. */
  async finish() {
    await expect(this.page.getByText('Site Config Created Successfully', { exact: true })).toBeVisible()
  }
}
