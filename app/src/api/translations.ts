import type { RouteLocationNormalizedLoadedGeneric } from 'vue-router'
import gettext from '@/gettext'

export default async function loadTranslations(route: RouteLocationNormalizedLoadedGeneric) {
  if (gettext.current !== 'en') {
    await fetch(`${import.meta.env.VITE_API_ROOT}/translation/${gettext.current}`).then(async r => {
      gettext.translations[gettext.current] = await r.json()
    })

    if (route?.meta?.name)
      document.title = `${route.meta.name?.()} | PrimeWaf`
  }

  watch(() => route, () => {
    if (route?.meta?.name)
      document.title = `${route.meta.name?.()} | PrimeWaf`
  })
}
