import type { Column } from '@/components/StdDesign/types'

export function getPithyColumns(columns: Column[]) {
  return columns.filter(c => {
    return c.pithy === true && !c.hiddenInTable
  })
}
