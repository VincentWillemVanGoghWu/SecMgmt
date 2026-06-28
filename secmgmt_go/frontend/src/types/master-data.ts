export interface FactoryRecord {
  id: number
  factoryCode: string
  factoryName: string
  status: string
  remark?: string | null
}

export interface ZoneRecord {
  id: number
  factoryId: number
  factoryName: string
  zoneCode: string
  zoneName: string
  status: string
  remark?: string | null
}

export interface DeptRecord {
  id: number
  deptCode: string
  deptName: string
  parentId?: number | null
  parentName?: string | null
  factoryId?: number | null
  factoryName?: string | null
  zoneId?: number | null
  zoneName?: string | null
  leader?: string | null
  phone?: string | null
  sort: number
  status: string
  remark?: string | null
}

export interface DictItemRecord {
  id: number
  dictTypeId: number
  itemLabel: string
  itemValue: string
  itemSort: number
  isDefault: boolean
  status: string
  remark?: string | null
}

export interface DictTypeRecord {
  id: number
  dictCode: string
  dictName: string
  status: string
  remark?: string | null
  items: DictItemRecord[]
}

export interface StatusPayload {
  status: string
}
