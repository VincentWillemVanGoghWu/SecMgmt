package service

import (
	"strings"

	"secmgmt_go/internal/domain/entity"
)

func (s *PlatformService) CreateFactory(payload FactoryPayload) (map[string]any, error) {
	factoryCode, err := s.ensureGeneratedCode("factory_area", "factory_code", strings.TrimSpace(payload.FactoryCode), "factory")
	if err != nil {
		return nil, err
	}
	item := entity.FactoryArea{
		FactoryCode: factoryCode,
		FactoryName: strings.TrimSpace(payload.FactoryName),
		Status:      normalizedStatus(payload.Status, "enabled"),
		Remark:      valueOrEmpty(payload.Remark),
	}
	if err := s.db().Create(&item).Error; err != nil {
		return nil, err
	}
	return map[string]any{"id": item.ID, "factoryCode": item.FactoryCode, "factoryName": item.FactoryName, "status": item.Status, "remark": nullableString(item.Remark)}, nil
}

func (s *PlatformService) UpdateFactory(factoryID uint, payload FactoryPayload) (map[string]any, error) {
	var item entity.FactoryArea
	if err := s.db().First(&item, factoryID).Error; err != nil {
		return nil, err
	}
	item.FactoryName = strings.TrimSpace(payload.FactoryName)
	item.Status = normalizedStatus(payload.Status, item.Status)
	item.Remark = valueOrEmpty(payload.Remark)
	if err := s.db().Save(item).Error; err != nil {
		return nil, err
	}
	return map[string]any{"id": item.ID, "factoryCode": item.FactoryCode, "factoryName": item.FactoryName, "status": item.Status, "remark": nullableString(item.Remark)}, nil
}

func (s *PlatformService) UpdateFactoryStatus(factoryID uint, payload StatusPayload) (map[string]any, error) {
	var item entity.FactoryArea
	if err := s.db().First(&item, factoryID).Error; err != nil {
		return nil, err
	}
	item.Status = normalizedStatus(payload.Status, item.Status)
	if err := s.db().Save(&item).Error; err != nil {
		return nil, err
	}
	return map[string]any{"id": item.ID, "factoryCode": item.FactoryCode, "factoryName": item.FactoryName, "status": item.Status, "remark": nullableString(item.Remark)}, nil
}

func (s *PlatformService) DeleteFactory(factoryID uint) error {
	return s.db().Delete(&entity.FactoryArea{}, factoryID).Error
}

func (s *PlatformService) CreateZone(payload ZonePayload) (map[string]any, error) {
	zoneCode, err := s.ensureGeneratedCode("factory_zone", "zone_code", strings.TrimSpace(payload.ZoneCode), "zone")
	if err != nil {
		return nil, err
	}
	item := entity.FactoryZone{
		FactoryID: payload.FactoryID,
		ZoneCode:  zoneCode,
		ZoneName:  strings.TrimSpace(payload.ZoneName),
		Status:    normalizedStatus(payload.Status, "enabled"),
		Remark:    valueOrEmpty(payload.Remark),
	}
	if err := s.db().Create(&item).Error; err != nil {
		return nil, err
	}
	return s.zoneRecord(item.ID)
}

func (s *PlatformService) UpdateZone(zoneID uint, payload ZonePayload) (map[string]any, error) {
	var item entity.FactoryZone
	if err := s.db().First(&item, zoneID).Error; err != nil {
		return nil, err
	}
	item.FactoryID = payload.FactoryID
	item.ZoneName = strings.TrimSpace(payload.ZoneName)
	item.Status = normalizedStatus(payload.Status, item.Status)
	item.Remark = valueOrEmpty(payload.Remark)
	if err := s.db().Save(&item).Error; err != nil {
		return nil, err
	}
	return s.zoneRecord(zoneID)
}

func (s *PlatformService) UpdateZoneStatus(zoneID uint, payload StatusPayload) (map[string]any, error) {
	var item entity.FactoryZone
	if err := s.db().First(&item, zoneID).Error; err != nil {
		return nil, err
	}
	item.Status = normalizedStatus(payload.Status, item.Status)
	if err := s.db().Save(&item).Error; err != nil {
		return nil, err
	}
	return s.zoneRecord(zoneID)
}

func (s *PlatformService) DeleteZone(zoneID uint) error {
	return s.db().Delete(&entity.FactoryZone{}, zoneID).Error
}

func (s *PlatformService) CreateDept(payload DeptPayload) (map[string]any, error) {
	deptCode, err := s.ensureGeneratedCode("sys_dept", "dept_code", strings.TrimSpace(payload.DeptCode), "dept")
	if err != nil {
		return nil, err
	}
	item := entity.SysDept{
		DeptCode:  deptCode,
		DeptName:  strings.TrimSpace(payload.DeptName),
		ParentID:  payload.ParentID,
		FactoryID: payload.FactoryID,
		ZoneID:    payload.ZoneID,
		Leader:    valueOrEmpty(payload.Leader),
		Phone:     valueOrEmpty(payload.Phone),
		Sort:      payload.Sort,
		Status:    normalizedStatus(payload.Status, "enabled"),
		Remark:    valueOrEmpty(payload.Remark),
	}
	if err := s.db().Create(&item).Error; err != nil {
		return nil, err
	}
	return s.deptRecord(item.ID)
}

func (s *PlatformService) UpdateDept(deptID uint, payload DeptPayload) (map[string]any, error) {
	var item entity.SysDept
	if err := s.db().First(&item, deptID).Error; err != nil {
		return nil, err
	}
	item.DeptName = strings.TrimSpace(payload.DeptName)
	item.ParentID = payload.ParentID
	item.FactoryID = payload.FactoryID
	item.ZoneID = payload.ZoneID
	item.Leader = valueOrEmpty(payload.Leader)
	item.Phone = valueOrEmpty(payload.Phone)
	item.Sort = payload.Sort
	item.Status = normalizedStatus(payload.Status, item.Status)
	item.Remark = valueOrEmpty(payload.Remark)
	if err := s.db().Save(&item).Error; err != nil {
		return nil, err
	}
	return s.deptRecord(deptID)
}

func (s *PlatformService) UpdateDeptStatus(deptID uint, payload StatusPayload) (map[string]any, error) {
	var item entity.SysDept
	if err := s.db().First(&item, deptID).Error; err != nil {
		return nil, err
	}
	item.Status = normalizedStatus(payload.Status, item.Status)
	if err := s.db().Save(&item).Error; err != nil {
		return nil, err
	}
	return s.deptRecord(deptID)
}

func (s *PlatformService) DeleteDept(deptID uint) error {
	return s.db().Delete(&entity.SysDept{}, deptID).Error
}

func (s *PlatformService) CreateDictType(payload DictTypePayload) (map[string]any, error) {
	item := entity.SysDictType{
		DictCode: strings.TrimSpace(payload.DictCode),
		DictName: strings.TrimSpace(payload.DictName),
		Status:   normalizedStatus(payload.Status, "enabled"),
		Remark:   valueOrEmpty(payload.Remark),
	}
	if err := s.db().Create(&item).Error; err != nil {
		return nil, err
	}
	return s.dictTypeRecord(item.ID)
}

func (s *PlatformService) UpdateDictType(dictTypeID uint, payload DictTypePayload) (map[string]any, error) {
	var item entity.SysDictType
	if err := s.db().First(&item, dictTypeID).Error; err != nil {
		return nil, err
	}
	item.DictCode = strings.TrimSpace(payload.DictCode)
	item.DictName = strings.TrimSpace(payload.DictName)
	item.Status = normalizedStatus(payload.Status, item.Status)
	item.Remark = valueOrEmpty(payload.Remark)
	if err := s.db().Save(&item).Error; err != nil {
		return nil, err
	}
	return s.dictTypeRecord(dictTypeID)
}

func (s *PlatformService) UpdateDictTypeStatus(dictTypeID uint, payload StatusPayload) (map[string]any, error) {
	var item entity.SysDictType
	if err := s.db().First(&item, dictTypeID).Error; err != nil {
		return nil, err
	}
	item.Status = normalizedStatus(payload.Status, item.Status)
	if err := s.db().Save(&item).Error; err != nil {
		return nil, err
	}
	return s.dictTypeRecord(dictTypeID)
}

func (s *PlatformService) DeleteDictType(dictTypeID uint) error {
	_ = s.db().Where("dict_type_id = ?", dictTypeID).Delete(&entity.SysDictItem{}).Error
	return s.db().Delete(&entity.SysDictType{}, dictTypeID).Error
}

func (s *PlatformService) CreateDictItem(payload DictItemPayload) (map[string]any, error) {
	item := entity.SysDictItem{
		DictTypeID: payload.DictTypeID,
		ItemLabel:  strings.TrimSpace(payload.ItemLabel),
		ItemValue:  strings.TrimSpace(payload.ItemValue),
		ItemSort:   payload.ItemSort,
		IsDefault:  payload.IsDefault,
		Status:     normalizedStatus(payload.Status, "enabled"),
		Remark:     valueOrEmpty(payload.Remark),
	}
	if err := s.db().Create(&item).Error; err != nil {
		return nil, err
	}
	return s.dictItemRecord(item.ID)
}

func (s *PlatformService) UpdateDictItem(itemID uint, payload DictItemPayload) (map[string]any, error) {
	var item entity.SysDictItem
	if err := s.db().First(&item, itemID).Error; err != nil {
		return nil, err
	}
	item.DictTypeID = payload.DictTypeID
	item.ItemLabel = strings.TrimSpace(payload.ItemLabel)
	item.ItemValue = strings.TrimSpace(payload.ItemValue)
	item.ItemSort = payload.ItemSort
	item.IsDefault = payload.IsDefault
	item.Status = normalizedStatus(payload.Status, item.Status)
	item.Remark = valueOrEmpty(payload.Remark)
	if err := s.db().Save(&item).Error; err != nil {
		return nil, err
	}
	return s.dictItemRecord(itemID)
}

func (s *PlatformService) UpdateDictItemStatus(itemID uint, payload StatusPayload) (map[string]any, error) {
	var item entity.SysDictItem
	if err := s.db().First(&item, itemID).Error; err != nil {
		return nil, err
	}
	item.Status = normalizedStatus(payload.Status, item.Status)
	if err := s.db().Save(&item).Error; err != nil {
		return nil, err
	}
	return s.dictItemRecord(itemID)
}

func (s *PlatformService) DeleteDictItem(itemID uint) error {
	return s.db().Delete(&entity.SysDictItem{}, itemID).Error
}
