import type { CosyError } from '@/lib/http/types'
import router from '@/routes'

/**
 * Saving configuration that loads code into the Nginx worker — the njs and
 * Perl directives, and the dynamic modules providing them — is refused by
 * default. An interactive administrator with a verified two-factor session
 * may still apply it by repeating the request with the opt-in, which is what
 * this composable asks for and does.
 *
 * The codes come from internal/config/errors.go.
 */
const CONFIG_ERROR_SCOPE = 'config'
const CONFIRM_REQUIRED_CODE = 40021
const TWO_FACTOR_REQUIRED_CODE = 40022

/**
 * Thrown when the save was not applied and the administrator has already been
 * told why: they dismissed the confirmation, or their account has no second
 * factor to confirm with. Callers skip their own error reporting for it, so a
 * dismissed dialog does not look like a failed save and does not discard the
 * content in the editor.
 */
export class RestrictedDirectivesNotAppliedError extends Error {
  constructor() {
    super('Restricted nginx directives were not applied')
    this.name = 'RestrictedDirectivesNotAppliedError'
  }
}

export function isRestrictedDirectivesNotApplied(error: unknown): boolean {
  return error instanceof RestrictedDirectivesNotAppliedError
}

interface RestrictedDirectiveRejection {
  directives: string[]
  files: string[]
}

function splitParam(param?: string): string[] {
  if (!param)
    return []
  return param.split(',').map(value => value.trim()).filter(Boolean)
}

/**
 * Reads the rejection out of whichever shape the error arrives in: the
 * interceptor hands callers the response body, while some paths keep the
 * axios error around it.
 */
function parseRejection(error: unknown, code: number): RestrictedDirectiveRejection | null {
  if (!error || typeof error !== 'object')
    return null

  const candidates: unknown[] = [error, (error as { response?: { data?: unknown } }).response?.data]
  for (const candidate of candidates) {
    if (!candidate || typeof candidate !== 'object')
      continue
    const cosyError = candidate as CosyError
    if (cosyError.scope !== CONFIG_ERROR_SCOPE || Number(cosyError.code) !== code)
      continue

    return {
      directives: splitParam(cosyError.params?.[0]),
      files: splitParam(cosyError.params?.[1]),
    }
  }

  return null
}

function listItems(values: string[]) {
  return h(
    'ul',
    { class: 'mb-2 pl-5 list-disc' },
    values.map(value => h('li', { class: 'font-mono text-sm break-all' }, value)),
  )
}

export function useRestrictedDirectives() {
  const { modal } = useGlobalApp()
  // The modal is opened from an async save handler, where the component
  // instance is no longer current, so the app context is captured here.
  const appContext = getCurrentInstance()?.appContext

  function confirm(rejection: RestrictedDirectiveRejection): Promise<void> {
    return new Promise((resolve, reject) => {
      modal.confirm({
        title: $gettext('Apply restricted nginx directives?'),
        centered: true,
        closable: true,
        keyboard: true,
        maskClosable: false,
        width: '520px',
        okText: $gettext('Apply anyway'),
        okType: 'danger',
        cancelText: $gettext('Cancel'),
        appContext,
        content: () => h('div', {}, [
          h(
            'p',
            { class: 'mb-2' },
            $gettext('This configuration loads code into the Nginx worker processes, which runs with the privileges of Nginx itself. Only apply it if you wrote these directives or trust where they came from.'),
          ),
          h('p', { class: 'mb-1 font-medium' }, $gettext('Restricted directives')),
          listItems(rejection.directives),
          h('p', { class: 'mb-1 font-medium' }, $gettext('Found in')),
          listItems(rejection.files),
          h(
            'p',
            { class: 'mb-0 opacity-70' },
            $gettext('Such a configuration is not replicated to other nodes. Save it on each node that needs it.'),
          ),
        ]),
        onOk: () => resolve(),
        onCancel: () => reject(new RestrictedDirectivesNotAppliedError()),
      })
    })
  }

  function guideTwoFactorEnrollment(rejection: RestrictedDirectiveRejection) {
    modal.confirm({
      title: $gettext('Two-factor authentication required'),
      centered: true,
      closable: true,
      width: '520px',
      okText: $gettext('Go to Settings'),
      cancelText: $gettext('Later'),
      appContext,
      content: () => h('div', {}, [
        h(
          'p',
          { class: 'mb-2' },
          $gettext('This configuration loads code into the Nginx worker processes. Applying it needs a two-factor confirmation, so enable two-factor authentication on your account first.'),
        ),
        listItems(rejection.directives),
      ]),
      onOk: () => router.push('/profile'),
    })
  }

  /**
   * Runs `save` and, when the server asks for the restricted-directive
   * confirmation, shows what would be applied and repeats the save with the
   * opt-in once the administrator accepts.
   *
   * The two-factor prompt itself is not opened here: these endpoints already
   * require a verified secure session, so the response interceptor has shown
   * it before this point. An account with no second factor is guided to
   * enrol one instead, since it has nothing to confirm with.
   */
  async function withConfirmation<T>(save: (allowRestrictedDirectives: boolean) => Promise<T>): Promise<T> {
    try {
      return await save(false)
    }
    catch (error) {
      const needsTwoFactor = parseRejection(error, TWO_FACTOR_REQUIRED_CODE)
      if (needsTwoFactor) {
        guideTwoFactorEnrollment(needsTwoFactor)
        throw new RestrictedDirectivesNotAppliedError()
      }

      const rejection = parseRejection(error, CONFIRM_REQUIRED_CODE)
      if (!rejection)
        throw error

      await confirm(rejection)

      return await save(true)
    }
  }

  return { withConfirmation }
}

export default useRestrictedDirectives
