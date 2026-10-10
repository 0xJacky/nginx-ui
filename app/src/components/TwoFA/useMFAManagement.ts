import twoFA from '@/api/2fa'
import { use2FAModal } from '@/components/TwoFA'

export function useMFAManagement() {
  const router = useRouter()
  const { message } = useGlobalApp()
  const modal = use2FAModal()

  return async () => {
    const status = await twoFA.status()
    if (!status.enabled) {
      message.warning($gettext('Enable MFA on your own account before managing MFA policies or resetting another user.'))
      await router.push({ path: '/profile', hash: '#two-factor-authentication' })
      return false
    }
    try {
      return !!await modal.open()
    }
    catch {
      return false
    }
  }
}
