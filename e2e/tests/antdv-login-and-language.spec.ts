import type { Page } from '@playwright/test'
import { expect, test } from '@playwright/test'
import { routeUrl } from './helpers'

interface LoginUiSnapshot {
  usernamePlaceholder: string | null
  passwordPlaceholder: string | null
  loginLabel: string
  appearanceTitle: string | null
}

interface LoginLayoutMeasurement {
  footer: { bottom: number, left: number, right: number, top: number }
  background: {
    bottom: number
    clientHeight: number
    left: number
    right: number
    scrollHeight: number
  }
}

async function readLoginUi(page: Page): Promise<LoginUiSnapshot> {
  return {
    usernamePlaceholder: await page.locator('#username').getAttribute('placeholder'),
    passwordPlaceholder: await page.locator('#password').getAttribute('placeholder'),
    loginLabel: (await page.locator('#components-form-demo-normal-login button[type="submit"]').innerText()).trim(),
    appearanceTitle: await page.locator('.VPSwitchAppearance').getAttribute('title'),
  }
}

function isExpectedDemoNetworkError(message: string) {
  return message.startsWith('Failed to load resource:')
    || /net::ERR_|NetworkError|network request failed/i.test(message)
}

async function measureLoginLayout(page: Page): Promise<LoginLayoutMeasurement> {
  return await page.locator('.footer').evaluate(footer => {
    const container = footer.closest('.ant-layout-content')
    if (!container)
      throw new Error('The login footer is not inside the page background container')

    const footerRect = footer.getBoundingClientRect()
    const backgroundRect = container.getBoundingClientRect()

    return {
      footer: {
        bottom: footerRect.bottom,
        left: footerRect.left,
        right: footerRect.right,
        top: footerRect.top,
      },
      background: {
        bottom: backgroundRect.bottom,
        clientHeight: container.clientHeight,
        left: backgroundRect.left,
        right: backgroundRect.right,
        scrollHeight: container.scrollHeight,
      },
    }
  })
}

test('login form, language selector, appearance switch, and footer survive antdv-next migration', async ({ page }) => {
  test.setTimeout(120_000)

  const unexpectedConsoleErrors: string[] = []
  const pageErrors: string[] = []

  page.on('console', message => {
    if (message.type() === 'error' && !isExpectedDemoNetworkError(message.text()))
      unexpectedConsoleErrors.push(message.text())
  })
  page.on('pageerror', error => pageErrors.push(error.message))

  // The shared storage state authenticates API requests, but clearing only the
  // persisted user store keeps this test on the public login screen.
  await page.addInitScript(() => localStorage.removeItem('user'))

  const response = await page.goto(routeUrl('/login'), { waitUntil: 'domcontentloaded' })
  if (response)
    expect(response.ok(), `Navigation to ${routeUrl('/login')} returned ${response.status()}`).toBe(true)

  await expect(page).toHaveURL(/#\/login(?:\?|$)/)
  const loginContainer = page.locator('.login-container')
  await expect(loginContainer).toBeVisible()

  const form = page.locator('#components-form-demo-normal-login')
  const username = page.locator('#username')
  const password = page.locator('#password')
  await expect(form).toBeVisible()
  await expect(username).toBeVisible()
  await expect(password).toBeVisible()
  await expect(form.locator('.ant-form-item')).toHaveCount(3)

  await password.fill('visibility-check')
  const passwordVisibilityButton = form.locator('.ant-form-item').filter({ has: password }).getByRole('button')
  await expect(passwordVisibilityButton).toBeVisible()
  await expect(password).toHaveAttribute('type', 'password')
  await passwordVisibilityButton.click()
  await expect(password).toHaveAttribute('type', 'text')
  await passwordVisibilityButton.click()
  await expect(password).toHaveAttribute('type', 'password')

  const languageSelect = page.locator('.footer .ant-select').first()
  const languageCombobox = languageSelect.getByRole('combobox')
  const languageContent = languageSelect.locator('.ant-select-content')
  await expect(languageSelect).toBeVisible()

  const initialLanguageLabel = (await languageContent.getAttribute('title') ?? '').trim()
  expect(initialLanguageLabel, 'The closed language Select has no visible label').not.toBe('')
  expect(initialLanguageLabel, 'The closed language Select shows a raw locale value').not.toMatch(/^[a-z]{2}(?:_[A-Z]{2})?$/)

  const initialLoginUi = await readLoginUi(page)
  await languageCombobox.click()

  const languageDropdown = page.locator('.ant-select-dropdown').last()
  await expect(languageDropdown).toBeVisible()
  await expect(languageDropdown).not.toHaveClass(/ant-select-dropdown-empty/)
  await expect(languageDropdown.locator('.ant-select-item-empty')).toHaveCount(0)

  const languageOptions = languageDropdown.locator('.ant-select-item-option')
  await expect.poll(() => languageOptions.count()).toBeGreaterThan(0)
  await expect(languageOptions.first()).toContainText(/\S/)
  const optionLabels = (await languageOptions.allTextContents()).map(label => label.trim()).filter(Boolean)
  expect(optionLabels).toContain(initialLanguageLabel)

  const alternativeLanguageOption = languageDropdown.locator('.ant-select-item-option:not(.ant-select-item-option-selected)').first()
  await expect(alternativeLanguageOption).toBeVisible()
  const alternativeLanguageLabel = (await alternativeLanguageOption.innerText()).trim()
  expect(alternativeLanguageLabel, 'The alternative language option has no human label').not.toBe('')
  expect(alternativeLanguageLabel, 'The language option shows a raw locale value').not.toMatch(/^[a-z]{2}(?:_[A-Z]{2})?$/)
  await alternativeLanguageOption.click()

  await expect(languageDropdown).toBeHidden()
  await expect(languageContent).toHaveAttribute('title', alternativeLanguageLabel)
  await expect(languageContent).toContainText(alternativeLanguageLabel)
  await expect.poll(async () => {
    const updatedLoginUi = await readLoginUi(page)
    return updatedLoginUi.usernamePlaceholder !== initialLoginUi.usernamePlaceholder
      || updatedLoginUi.passwordPlaceholder !== initialLoginUi.passwordPlaceholder
      || updatedLoginUi.loginLabel !== initialLoginUi.loginLabel
      || updatedLoginUi.appearanceTitle !== initialLoginUi.appearanceTitle
  }).toBe(true)

  const appearanceSwitch = page.locator('.VPSwitchAppearance')
  await expect(appearanceSwitch).toBeVisible()
  const initialThemeState = await appearanceSwitch.getAttribute('aria-checked')
  expect(initialThemeState === 'true' || initialThemeState === 'false').toBe(true)
  const expectedThemeState = initialThemeState === 'true' ? 'false' : 'true'
  const initialBodyClass = await page.locator('body').getAttribute('class')
  const initialAppearanceTitle = await appearanceSwitch.getAttribute('title')

  await appearanceSwitch.click()
  await expect(appearanceSwitch).toHaveAttribute('aria-checked', expectedThemeState)
  await expect.poll(() => page.locator('body').getAttribute('class')).not.toBe(initialBodyClass)
  await expect.poll(() => appearanceSwitch.getAttribute('title')).not.toBe(initialAppearanceTitle)
  const expectedThemeClass = expectedThemeState === 'true' ? 'dark' : 'light'
  await expect.poll(async () => (await page.locator('body').getAttribute('class') ?? '').split(/\s+/).includes(expectedThemeClass)).toBe(true)

  // Force the content to exceed a short viewport and verify the background
  // grows with the footer instead of clipping it at 100vh.
  await page.setViewportSize({ width: 1440, height: 400 })
  const layout = await measureLoginLayout(page)
  expect(layout.footer.top).toBeGreaterThanOrEqual(-1)
  expect(layout.footer.bottom).toBeLessThanOrEqual(layout.background.bottom + 1)
  expect(layout.footer.left).toBeGreaterThanOrEqual(layout.background.left - 1)
  expect(layout.footer.right).toBeLessThanOrEqual(layout.background.right + 1)
  expect(layout.background.scrollHeight).toBeLessThanOrEqual(layout.background.clientHeight + 1)

  expect(unexpectedConsoleErrors, `Unexpected browser console errors: ${unexpectedConsoleErrors.join('\n')}`).toEqual([])
  expect(pageErrors, `Browser page errors: ${pageErrors.join('\n')}`).toEqual([])
})
