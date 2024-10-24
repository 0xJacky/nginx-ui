import type { Bread } from '@/components/Breadcrumb/types'

export function useBreadcrumbs() {
  return inject('breadList') as Ref<Bread[]>
}
