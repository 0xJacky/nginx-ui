import type { DefaultOptionType } from 'ant-design-vue/es/vc-cascader'

export interface Author {
  id?: number
  name: string
  checked?: boolean
  sort?: number
  affiliated_unit?: string
}

export interface AuthorSelector {
  input?: {
    title?: () => string
    placeholder?: () => string
  }
  checkbox?: {
    title?: () => string
    placeholder?: () => string
  }
  select?: {
    title?: () => string
    placeholder?: () => string
    options?: DefaultOptionType[]
  }
}
