export interface RouteHashScrollOptions {
  headerSelector?: string
  offset?: number
}

export function useRouteHashScroll(options: RouteHashScrollOptions = {}) {
  const route = useRoute()
  const headerSelector = options.headerSelector ?? '.ant-layout-header'
  const offset = options.offset ?? 16
  let scrollFrame: number | undefined

  function getHashTarget() {
    if (!route.hash)
      return null

    try {
      return document.getElementById(decodeURIComponent(route.hash.slice(1)))
    }
    catch {
      return null
    }
  }

  function scrollToHash() {
    const target = getHashTarget()
    if (!target)
      return false

    const headerHeight = document.querySelector<HTMLElement>(headerSelector)?.offsetHeight ?? 0
    const targetTop = window.scrollY + target.getBoundingClientRect().top - headerHeight - offset
    window.scrollTo({ top: Math.max(0, targetTop) })
    return true
  }

  function cancelScheduledScroll() {
    if (scrollFrame !== undefined) {
      cancelAnimationFrame(scrollFrame)
      scrollFrame = undefined
    }
  }

  async function scheduleHashScroll() {
    cancelScheduledScroll()
    await nextTick()

    scrollFrame = requestAnimationFrame(() => {
      scrollFrame = undefined
      scrollToHash()
    })
  }

  function handleRouteEnter() {
    if (route.hash)
      void scheduleHashScroll()
  }

  watch([() => route.path, () => route.hash], ([path, hash], [previousPath]) => {
    if (!hash) {
      cancelScheduledScroll()
      return
    }

    if (path === previousPath)
      void scheduleHashScroll()
  })

  onMounted(() => {
    if (route.hash)
      void scheduleHashScroll()
  })

  onBeforeUnmount(cancelScheduledScroll)

  return {
    handleRouteEnter,
    scheduleHashScroll,
    scrollToHash,
  }
}
