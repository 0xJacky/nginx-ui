import type { Bread } from '@/components/Breadcrumb/types'

export const useBreadcrumbs = () => {
  return inject('breadList') as Ref<Bread[]>
}
