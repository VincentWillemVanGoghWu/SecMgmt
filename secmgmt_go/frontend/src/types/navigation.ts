export interface MenuItem {
  id?: number
  key: string
  label: string
  icon?: string
  routeName?: string
  path?: string
  children?: MenuItem[]
}

export type StatusTone = 'success' | 'warning' | 'danger' | 'info' | 'default'

export interface StatusValue {
  text: string
  tone: StatusTone
}

export interface TableColumn {
  key: string
  label: string
}

export interface ModulePageConfig {
  title: string
  description: string
  filters: string[]
  columns: TableColumn[]
  rows: Array<Record<string, string | number | StatusValue>>
}
