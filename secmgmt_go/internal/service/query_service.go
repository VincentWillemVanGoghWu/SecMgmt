package service

import (
	"strings"
	"time"

	"secmgmt_go/internal/domain/dto"
	"secmgmt_go/internal/repository"
)

type FactoryListFilter struct {
	Keyword string
	Status  string
}

type ZoneListFilter struct {
	Keyword   string
	Status    string
	FactoryID uint
}

type DeptListFilter struct {
	Keyword   string
	Status    string
	FactoryID uint
}

type DictTypeListFilter struct {
	Keyword string
	Status  string
}

type CameraListFilter struct {
	Keyword   string
	FactoryID uint
	ZoneID    uint
	Status    string
	SupportAI *bool
}

type RecorderListFilter struct {
	Keyword   string
	FactoryID uint
	Status    string
}

type ChannelListFilter struct {
	Keyword   string
	FactoryID uint
	ZoneID    uint
	Status    string
}

type QueryService struct {
	repo *repository.Repository
}

func NewQueryService(repo *repository.Repository) *QueryService {
	return &QueryService{repo: repo}
}

func (s *QueryService) ListFactories(filter FactoryListFilter) ([]dto.FactoryRecord, error) {
	items, err := s.repo.ListFactories()
	if err != nil {
		return nil, err
	}

	result := make([]dto.FactoryRecord, 0, len(items))
	for _, item := range items {
		if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
			if !containsFold(item.FactoryCode, keyword) && !containsFold(item.FactoryName, keyword) {
				continue
			}
		}
		if filter.Status != "" && item.Status != filter.Status {
			continue
		}
		result = append(result, dto.FactoryRecord{
			ID:          item.ID,
			FactoryCode: item.FactoryCode,
			FactoryName: item.FactoryName,
			Status:      item.Status,
			Remark:      stringPtr(item.Remark),
		})
	}
	return result, nil
}

func (s *QueryService) ListZones(filter ZoneListFilter) ([]dto.ZoneRecord, error) {
	items, err := s.repo.ListZones()
	if err != nil {
		return nil, err
	}

	result := make([]dto.ZoneRecord, 0, len(items))
	for _, item := range items {
		if filter.FactoryID > 0 && item.FactoryID != filter.FactoryID {
			continue
		}
		if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
			if !containsFold(item.ZoneCode, keyword) && !containsFold(item.ZoneName, keyword) {
				continue
			}
		}
		if filter.Status != "" && item.Status != filter.Status {
			continue
		}
		result = append(result, dto.ZoneRecord{
			ID:          item.ID,
			FactoryID:   item.FactoryID,
			FactoryName: item.FactoryName,
			ZoneCode:    item.ZoneCode,
			ZoneName:    item.ZoneName,
			Status:      item.Status,
			Remark:      stringPtr(item.Remark),
		})
	}
	return result, nil
}

func (s *QueryService) ListDepts(filter DeptListFilter) ([]dto.DeptRecord, error) {
	items, err := s.repo.ListDepts()
	if err != nil {
		return nil, err
	}

	result := make([]dto.DeptRecord, 0, len(items))
	for _, item := range items {
		if filter.FactoryID > 0 {
			if item.FactoryID == nil || *item.FactoryID != filter.FactoryID {
				continue
			}
		}
		if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
			if !containsFold(item.DeptCode, keyword) && !containsFold(item.DeptName, keyword) {
				continue
			}
		}
		if filter.Status != "" && item.Status != filter.Status {
			continue
		}
		result = append(result, dto.DeptRecord{
			ID:          item.ID,
			DeptCode:    item.DeptCode,
			DeptName:    item.DeptName,
			ParentID:    item.ParentID,
			ParentName:  item.ParentName,
			FactoryID:   item.FactoryID,
			FactoryName: item.FactoryName,
			ZoneID:      item.ZoneID,
			ZoneName:    item.ZoneName,
			Leader:      stringPtr(item.Leader),
			Phone:       stringPtr(item.Phone),
			Sort:        item.Sort,
			Status:      item.Status,
			Remark:      stringPtr(item.Remark),
		})
	}
	return result, nil
}

func (s *QueryService) ListDictTypes(filter DictTypeListFilter) ([]dto.DictTypeRecord, error) {
	types, err := s.repo.ListDictTypes()
	if err != nil {
		return nil, err
	}
	items, err := s.repo.ListDictItems()
	if err != nil {
		return nil, err
	}

	itemMap := make(map[uint][]dto.DictItemRecord)
	for _, item := range items {
		itemMap[item.DictTypeID] = append(itemMap[item.DictTypeID], dto.DictItemRecord{
			ID:         item.ID,
			DictTypeID: item.DictTypeID,
			ItemLabel:  item.ItemLabel,
			ItemValue:  item.ItemValue,
			ItemSort:   item.ItemSort,
			IsDefault:  item.IsDefault,
			Status:     item.Status,
			Remark:     stringPtr(item.Remark),
		})
	}

	result := make([]dto.DictTypeRecord, 0, len(types))
	for _, item := range types {
		if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
			if !containsFold(item.DictCode, keyword) && !containsFold(item.DictName, keyword) {
				continue
			}
		}
		if filter.Status != "" && item.Status != filter.Status {
			continue
		}
		result = append(result, dto.DictTypeRecord{
			ID:       item.ID,
			DictCode: item.DictCode,
			DictName: item.DictName,
			Status:   item.Status,
			Remark:   stringPtr(item.Remark),
			Items:    itemMap[item.ID],
		})
	}

	return result, nil
}

func (s *QueryService) ListCameras(filter CameraListFilter) ([]dto.CameraRecord, error) {
	items, err := s.repo.ListCameras()
	if err != nil {
		return nil, err
	}

	result := make([]dto.CameraRecord, 0, len(items))
	for _, item := range items {
		if filter.FactoryID > 0 && item.FactoryID != filter.FactoryID {
			continue
		}
		if filter.ZoneID > 0 && item.ZoneID != filter.ZoneID {
			continue
		}
		if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
			if !containsFold(item.DeviceCode, keyword) && !containsFold(item.Name, keyword) && !containsFold(item.IP, keyword) {
				continue
			}
		}
		if filter.Status != "" && item.Status != filter.Status {
			continue
		}
		if filter.SupportAI != nil && item.SupportAI != *filter.SupportAI {
			continue
		}
		result = append(result, dto.CameraRecord{
			ID:                 item.ID,
			DeviceCode:         item.DeviceCode,
			Name:               item.Name,
			IP:                 item.IP,
			SDKPort:            item.SDKPort,
			HTTPPort:           item.HTTPPort,
			RTSPPort:           item.RTSPPort,
			Username:           item.Username,
			FactoryID:          item.FactoryID,
			FactoryName:        item.FactoryName,
			ZoneID:             item.ZoneID,
			ZoneName:           item.ZoneName,
			InstallLocation:    stringPtr(item.InstallLocation),
			SupportAI:          item.SupportAI,
			Status:             item.Status,
			LastOnlineAt:       timePtrToString(item.LastOnlineAt),
			Remark:             stringPtr(item.Remark),
			PasswordConfigured: item.PasswordEncrypted != "",
		})
	}
	return result, nil
}

func (s *QueryService) ListRecorders(filter RecorderListFilter) ([]dto.RecorderRecord, error) {
	items, err := s.repo.ListRecorders()
	if err != nil {
		return nil, err
	}

	result := make([]dto.RecorderRecord, 0, len(items))
	for _, item := range items {
		if filter.FactoryID > 0 && item.FactoryID != filter.FactoryID {
			continue
		}
		if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
			if !containsFold(item.DeviceCode, keyword) && !containsFold(item.Name, keyword) && !containsFold(item.IP, keyword) {
				continue
			}
		}
		if filter.Status != "" && item.Status != filter.Status {
			continue
		}
		result = append(result, dto.RecorderRecord{
			ID:                 item.ID,
			DeviceCode:         item.DeviceCode,
			Name:               item.Name,
			IP:                 item.IP,
			SDKPort:            item.SDKPort,
			HTTPPort:           item.HTTPPort,
			Username:           item.Username,
			ChannelCount:       item.ChannelCount,
			FactoryID:          item.FactoryID,
			FactoryName:        item.FactoryName,
			Status:             item.Status,
			LastOnlineAt:       timePtrToString(item.LastOnlineAt),
			PasswordConfigured: item.PasswordEncrypted != "",
		})
	}
	return result, nil
}

func (s *QueryService) ListChannels(filter ChannelListFilter) ([]dto.RecorderChannelRecord, error) {
	items, err := s.repo.ListChannels()
	if err != nil {
		return nil, err
	}

	result := make([]dto.RecorderChannelRecord, 0, len(items))
	for _, item := range items {
		if filter.FactoryID > 0 && item.FactoryID != filter.FactoryID {
			continue
		}
		if filter.ZoneID > 0 {
			if item.ZoneID == nil || *item.ZoneID != filter.ZoneID {
				continue
			}
		}
		if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
			if !containsFold(item.Name, keyword) && !containsFold(item.RecorderName, keyword) && !containsFold(nullableValue(item.CameraName), keyword) {
				continue
			}
		}
		if filter.Status != "" && item.Status != filter.Status {
			continue
		}
		result = append(result, dto.RecorderChannelRecord{
			ID:              item.ID,
			RecorderID:      item.RecorderID,
			RecorderName:    item.RecorderName,
			ChannelNo:       item.ChannelNo,
			Name:            item.Name,
			CameraID:        item.CameraID,
			CameraName:      item.CameraName,
			FactoryID:       item.FactoryID,
			FactoryName:     item.FactoryName,
			ZoneID:          item.ZoneID,
			ZoneName:        item.ZoneName,
			Enabled:         item.Enabled,
			SupportPlayback: item.SupportPlayback,
			Status:          item.Status,
		})
	}
	return result, nil
}

func (s *QueryService) ListRealtimeAlarms(page, pageSize int, filter dto.AlarmListFilter) (*dto.AlarmRealtimePageRecord, error) {
	filter.ExcludeDone = true
	if filter.StartAt == nil {
		now := time.Now()
		startAt := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -2)
		filter.StartAt = &startAt
	}
	items, total, err := s.repo.ListAlarms(page, pageSize, filter)
	if err != nil {
		return nil, err
	}

	result := &dto.AlarmRealtimePageRecord{
		Items:    mapAlarmRows(items),
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}
	return result, nil
}

func (s *QueryService) ListAlarms(page, pageSize int, filter dto.AlarmListFilter) (*dto.AlarmPageRecord, error) {
	items, total, err := s.repo.ListAlarms(page, pageSize, filter)
	if err != nil {
		return nil, err
	}

	return &dto.AlarmPageRecord{
		Items:    mapAlarmRows(items),
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func (s *QueryService) GetDashboardSummary(startAt, endAt *time.Time) (*dto.DashboardSummary, error) {
	item, err := s.repo.GetDashboardSummary(startAt, endAt)
	if err != nil {
		return nil, err
	}

	cameraOnlineRate := 0.0
	if item.CameraTotalCount > 0 {
		cameraOnlineRate = float64(item.CameraOnlineCount) / float64(item.CameraTotalCount) * 100
	}

	recorderOnlineRate := 0.0
	if item.RecorderTotalCount > 0 {
		recorderOnlineRate = float64(item.RecorderOnlineCount) / float64(item.RecorderTotalCount) * 100
	}

	pushSuccessRate := 0.0
	pushTotalCount := item.PushSuccessCount + item.PushFailedCount
	if pushTotalCount > 0 {
		pushSuccessRate = float64(item.PushSuccessCount) / float64(pushTotalCount) * 100
	}

	return &dto.DashboardSummary{
		TodayAlarmCount:     item.TodayAlarmCount,
		PendingAlarmCount:   item.PendingAlarmCount,
		CriticalAlarmCount:  item.CriticalAlarmCount,
		CameraOnlineRate:    cameraOnlineRate,
		RecorderOnlineRate:  recorderOnlineRate,
		PushSuccessRate:     pushSuccessRate,
		CameraOnlineCount:   item.CameraOnlineCount,
		CameraTotalCount:    item.CameraTotalCount,
		RecorderOnlineCount: item.RecorderOnlineCount,
		RecorderTotalCount:  item.RecorderTotalCount,
	}, nil
}

func mapAlarmRows(items []repository.AlarmRow) []dto.AlarmRecord {
	result := make([]dto.AlarmRecord, 0, len(items))
	for _, item := range items {
		result = append(result, dto.AlarmRecord{
			ID:              item.ID,
			AlarmNo:         item.AlarmNo,
			AIEventID:       item.AIEventID,
			AlarmType:       item.AlarmType,
			AlarmLevel:      item.AlarmLevel,
			AlarmTime:       item.AlarmTime.Format(time.RFC3339),
			Status:          item.Status,
			CameraID:        item.CameraID,
			CameraName:      item.CameraName,
			RecorderID:      item.RecorderID,
			RecorderName:    item.RecorderName,
			ChannelID:       item.ChannelID,
			ChannelName:     item.ChannelName,
			FactoryID:       item.FactoryID,
			FactoryName:     item.FactoryName,
			ZoneID:          item.ZoneID,
			ZoneName:        item.ZoneName,
			Message:         stringPtr(item.Message),
			ImageURL:        stringPtr(item.ImageURL),
			VideoURL:        stringPtr(item.VideoURL),
			RecordStartTime: timePtrToString(item.RecordStartTime),
			RecordEndTime:   timePtrToString(item.RecordEndTime),
			OccurrenceCount: item.OccurrenceCount,
			LastEventTime:   timePtrToString(item.LastEventTime),
			CreatedAt:       item.CreatedAt.Format(time.RFC3339),
		})
	}
	return result
}

func timePtrToString(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.Format(time.RFC3339)
	return &formatted
}

func containsFold(value, keyword string) bool {
	return strings.Contains(strings.ToLower(value), strings.ToLower(keyword))
}

func nullableValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func stringPtr(value string) *string {
	if value == "" {
		return nil
	}
	copyValue := value
	return &copyValue
}
