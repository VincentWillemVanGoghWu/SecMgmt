package service

import (
	"encoding/json"
	"sort"
	"strings"

	"secmgmt_go/internal/domain/dto"
	"secmgmt_go/internal/domain/entity"
)

type AccessScope struct {
	All         bool
	UserID      uint
	DeptIDs     []uint
	UserIDs     []uint
	FactoryIDs  []uint
	ZoneIDs     []uint
	CameraIDs   []uint
	RecorderIDs []uint
	ChannelIDs  []uint
	SelfOnly    bool
}

type customAccessScopeValue struct {
	DeptIDs     []uint `json:"deptIds"`
	UserIDs     []uint `json:"userIds"`
	FactoryIDs  []uint `json:"factoryIds"`
	ZoneIDs     []uint `json:"zoneIds"`
	CameraIDs   []uint `json:"cameraIds"`
	RecorderIDs []uint `json:"recorderIds"`
	ChannelIDs  []uint `json:"channelIds"`
}

func BuildAccessScope(roles []entity.Role, user *entity.User, dept *entity.SysDept) *AccessScope {
	scope := &AccessScope{}
	if user != nil {
		scope.UserID = user.ID
	}
	for _, role := range roles {
		scope.mergeRole(role, user, dept)
		if scope.All {
			break
		}
	}
	scope.normalize()
	return scope
}

func (s *AccessScope) mergeRole(role entity.Role, user *entity.User, dept *entity.SysDept) {
	if s == nil {
		return
	}
	switch strings.ToLower(strings.TrimSpace(role.DataScopeType)) {
	case "", "all":
		s.All = true
	case "factory":
		s.FactoryIDs = appendUniqueUint(s.FactoryIDs, parseUintListJSON(role.DataScopeValue)...)
	case "zone":
		s.ZoneIDs = appendUniqueUint(s.ZoneIDs, parseUintListJSON(role.DataScopeValue)...)
	case "device":
		deviceIDs := parseUintListJSON(role.DataScopeValue)
		s.CameraIDs = appendUniqueUint(s.CameraIDs, deviceIDs...)
		s.RecorderIDs = appendUniqueUint(s.RecorderIDs, deviceIDs...)
		s.ChannelIDs = appendUniqueUint(s.ChannelIDs, deviceIDs...)
	case "dept":
		if dept != nil {
			s.DeptIDs = appendUniqueUint(s.DeptIDs, dept.ID)
			if dept.FactoryID != nil {
				s.FactoryIDs = appendUniqueUint(s.FactoryIDs, *dept.FactoryID)
			}
			if dept.ZoneID != nil {
				s.ZoneIDs = appendUniqueUint(s.ZoneIDs, *dept.ZoneID)
			}
		} else if user != nil && user.DeptID != nil {
			s.DeptIDs = appendUniqueUint(s.DeptIDs, *user.DeptID)
		}
	case "self":
		s.SelfOnly = true
		if user != nil {
			s.UserIDs = appendUniqueUint(s.UserIDs, user.ID)
		}
	case "custom":
		var custom customAccessScopeValue
		if err := json.Unmarshal([]byte(strings.TrimSpace(role.DataScopeValue)), &custom); err == nil {
			s.DeptIDs = appendUniqueUint(s.DeptIDs, custom.DeptIDs...)
			s.UserIDs = appendUniqueUint(s.UserIDs, custom.UserIDs...)
			s.FactoryIDs = appendUniqueUint(s.FactoryIDs, custom.FactoryIDs...)
			s.ZoneIDs = appendUniqueUint(s.ZoneIDs, custom.ZoneIDs...)
			s.CameraIDs = appendUniqueUint(s.CameraIDs, custom.CameraIDs...)
			s.RecorderIDs = appendUniqueUint(s.RecorderIDs, custom.RecorderIDs...)
			s.ChannelIDs = appendUniqueUint(s.ChannelIDs, custom.ChannelIDs...)
		}
	}
}

func (s *AccessScope) normalize() {
	if s == nil {
		return
	}
	if s.All {
		s.SelfOnly = false
		s.DeptIDs = nil
		s.UserIDs = nil
		s.FactoryIDs = nil
		s.ZoneIDs = nil
		s.CameraIDs = nil
		s.RecorderIDs = nil
		s.ChannelIDs = nil
		return
	}
	sortUintAsc(s.DeptIDs)
	sortUintAsc(s.UserIDs)
	sortUintAsc(s.FactoryIDs)
	sortUintAsc(s.ZoneIDs)
	sortUintAsc(s.CameraIDs)
	sortUintAsc(s.RecorderIDs)
	sortUintAsc(s.ChannelIDs)
}

func (s *AccessScope) ApplyToAlarmFilter(filter *dto.AlarmListFilter) {
	if s == nil || filter == nil || s.All {
		return
	}
	filter.FactoryIDs = appendUniqueUint(filter.FactoryIDs, s.FactoryIDs...)
	filter.ZoneIDs = appendUniqueUint(filter.ZoneIDs, s.ZoneIDs...)
	filter.CameraIDs = appendUniqueUint(filter.CameraIDs, s.CameraIDs...)
	filter.RecorderIDs = appendUniqueUint(filter.RecorderIDs, s.RecorderIDs...)
	filter.ChannelIDs = appendUniqueUint(filter.ChannelIDs, s.ChannelIDs...)
	filter.DenyAll = !s.All &&
		len(filter.FactoryIDs) == 0 &&
		len(filter.ZoneIDs) == 0 &&
		len(filter.CameraIDs) == 0 &&
		len(filter.RecorderIDs) == 0 &&
		len(filter.ChannelIDs) == 0
}

func (s *AccessScope) ToFilter() dto.AccessScopeFilter {
	if s == nil {
		return dto.AccessScopeFilter{}
	}
	return dto.AccessScopeFilter{
		All:         s.All,
		FactoryIDs:  append([]uint(nil), s.FactoryIDs...),
		ZoneIDs:     append([]uint(nil), s.ZoneIDs...),
		CameraIDs:   append([]uint(nil), s.CameraIDs...),
		RecorderIDs: append([]uint(nil), s.RecorderIDs...),
		ChannelIDs:  append([]uint(nil), s.ChannelIDs...),
	}
}

func (s *AccessScope) AllowsFactory(factoryID uint) bool {
	if s == nil || s.All {
		return true
	}
	return containsScopeUint(s.FactoryIDs, factoryID)
}

func (s *AccessScope) AllowsZone(factoryID uint, zoneID uint) bool {
	if s == nil || s.All {
		return true
	}
	return containsScopeUint(s.ZoneIDs, zoneID) || containsScopeUint(s.FactoryIDs, factoryID)
}

func (s *AccessScope) AllowsCamera(factoryID, zoneID, cameraID uint) bool {
	if s == nil || s.All {
		return true
	}
	if len(s.CameraIDs) > 0 {
		return containsScopeUint(s.CameraIDs, cameraID)
	}
	if len(s.RecorderIDs) > 0 || len(s.ChannelIDs) > 0 {
		return false
	}
	if len(s.ZoneIDs) > 0 {
		return containsScopeUint(s.ZoneIDs, zoneID)
	}
	if len(s.FactoryIDs) > 0 {
		return containsScopeUint(s.FactoryIDs, factoryID)
	}
	return false
}

func (s *AccessScope) AllowsRecorder(factoryID, recorderID uint) bool {
	if s == nil || s.All {
		return true
	}
	if len(s.RecorderIDs) > 0 {
		return containsScopeUint(s.RecorderIDs, recorderID)
	}
	if len(s.ChannelIDs) > 0 {
		return false
	}
	if len(s.FactoryIDs) > 0 {
		return containsScopeUint(s.FactoryIDs, factoryID)
	}
	return false
}

func (s *AccessScope) AllowsChannel(factoryID uint, zoneID, cameraID *uint, recorderID, channelID uint) bool {
	if s == nil || s.All {
		return true
	}
	if len(s.ChannelIDs) > 0 {
		return containsScopeUint(s.ChannelIDs, channelID)
	}
	if len(s.RecorderIDs) > 0 {
		return containsScopeUint(s.RecorderIDs, recorderID)
	}
	if len(s.CameraIDs) > 0 {
		return cameraID != nil && containsScopeUint(s.CameraIDs, *cameraID)
	}
	if len(s.ZoneIDs) > 0 {
		return zoneID != nil && containsScopeUint(s.ZoneIDs, *zoneID)
	}
	if len(s.FactoryIDs) > 0 {
		return containsScopeUint(s.FactoryIDs, factoryID)
	}
	return false
}

func (s *AccessScope) AllowsAlarm(factoryID, zoneID, cameraID, recorderID, channelID *uint) bool {
	if s == nil || s.All {
		return true
	}
	if len(s.ChannelIDs) > 0 {
		return channelID != nil && containsScopeUint(s.ChannelIDs, *channelID)
	}
	if len(s.RecorderIDs) > 0 {
		return recorderID != nil && containsScopeUint(s.RecorderIDs, *recorderID)
	}
	if len(s.CameraIDs) > 0 {
		return cameraID != nil && containsScopeUint(s.CameraIDs, *cameraID)
	}
	if len(s.ZoneIDs) > 0 {
		return zoneID != nil && containsScopeUint(s.ZoneIDs, *zoneID)
	}
	if len(s.FactoryIDs) > 0 {
		return factoryID != nil && containsScopeUint(s.FactoryIDs, *factoryID)
	}
	return false
}

func parseUintListJSON(raw string) []uint {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var values []uint
	if err := json.Unmarshal([]byte(raw), &values); err == nil {
		return values
	}
	var generic []any
	if err := json.Unmarshal([]byte(raw), &generic); err != nil {
		return nil
	}
	result := make([]uint, 0, len(generic))
	for _, item := range generic {
		switch value := item.(type) {
		case float64:
			result = appendUniqueUint(result, uint(value))
		}
	}
	return result
}

func appendUniqueUint(base []uint, values ...uint) []uint {
	for _, value := range values {
		if value == 0 || containsScopeUint(base, value) {
			continue
		}
		base = append(base, value)
	}
	return base
}

func containsScopeUint(values []uint, target uint) bool {
	if target == 0 {
		return false
	}
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func sortUintAsc(values []uint) {
	sort.Slice(values, func(i, j int) bool {
		return values[i] < values[j]
	})
}
