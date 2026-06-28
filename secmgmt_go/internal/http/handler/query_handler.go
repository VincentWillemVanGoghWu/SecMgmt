package handler

import (
	"strconv"
	"strings"

	"secmgmt_go/internal/domain/dto"
	"secmgmt_go/internal/http/response"
	"secmgmt_go/internal/service"

	"github.com/gin-gonic/gin"
)

type QueryHandler struct {
	queryService *service.QueryService
}

func NewQueryHandler(queryService *service.QueryService) *QueryHandler {
	return &QueryHandler{queryService: queryService}
}

func (h *QueryHandler) Health(c *gin.Context) {
	response.OK(c, gin.H{
		"status":  "ok",
		"service": "secmgmt-go",
	})
}

func (h *QueryHandler) ListFactories(c *gin.Context) {
	data, err := h.queryService.ListFactories(service.FactoryListFilter{
		Keyword: strings.TrimSpace(c.Query("keyword")),
		Status:  strings.TrimSpace(c.Query("status")),
	})
	if err != nil {
		response.Error(c, 500, err.Error())
		return
	}
	response.OK(c, data)
}

func (h *QueryHandler) ListZones(c *gin.Context) {
	factoryID, err := readOptionalUintQuery(c, "factory_id")
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	data, err := h.queryService.ListZones(service.ZoneListFilter{
		Keyword:   strings.TrimSpace(c.Query("keyword")),
		Status:    strings.TrimSpace(c.Query("status")),
		FactoryID: factoryID,
	})
	if err != nil {
		response.Error(c, 500, err.Error())
		return
	}
	response.OK(c, data)
}

func (h *QueryHandler) ListDepts(c *gin.Context) {
	factoryID, err := readOptionalUintQuery(c, "factory_id")
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	data, err := h.queryService.ListDepts(service.DeptListFilter{
		Keyword:   strings.TrimSpace(c.Query("keyword")),
		Status:    strings.TrimSpace(c.Query("status")),
		FactoryID: factoryID,
	})
	if err != nil {
		response.Error(c, 500, err.Error())
		return
	}
	response.OK(c, data)
}

func (h *QueryHandler) ListDictTypes(c *gin.Context) {
	data, err := h.queryService.ListDictTypes(service.DictTypeListFilter{
		Keyword: strings.TrimSpace(c.Query("keyword")),
		Status:  strings.TrimSpace(c.Query("status")),
	})
	if err != nil {
		response.Error(c, 500, err.Error())
		return
	}
	response.OK(c, data)
}

func (h *QueryHandler) ListCameras(c *gin.Context) {
	factoryID, err := readOptionalUintQuery(c, "factory_id")
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	zoneID, err := readOptionalUintQuery(c, "zone_id")
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	supportAI, err := readOptionalBoolQuery(c, "support_ai")
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	data, err := h.queryService.ListCameras(service.CameraListFilter{
		Keyword:   strings.TrimSpace(c.Query("keyword")),
		FactoryID: factoryID,
		ZoneID:    zoneID,
		Status:    strings.TrimSpace(c.Query("status")),
		SupportAI: supportAI,
	})
	if err != nil {
		response.Error(c, 500, err.Error())
		return
	}
	response.OK(c, data)
}

func (h *QueryHandler) ListRecorders(c *gin.Context) {
	factoryID, err := readOptionalUintQuery(c, "factory_id")
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	data, err := h.queryService.ListRecorders(service.RecorderListFilter{
		Keyword:   strings.TrimSpace(c.Query("keyword")),
		FactoryID: factoryID,
		Status:    strings.TrimSpace(c.Query("status")),
	})
	if err != nil {
		response.Error(c, 500, err.Error())
		return
	}
	response.OK(c, data)
}

func (h *QueryHandler) ListChannels(c *gin.Context) {
	factoryID, err := readOptionalUintQuery(c, "factory_id")
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	zoneID, err := readOptionalUintQuery(c, "zone_id")
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	data, err := h.queryService.ListChannels(service.ChannelListFilter{
		Keyword:   strings.TrimSpace(c.Query("keyword")),
		FactoryID: factoryID,
		ZoneID:    zoneID,
		Status:    strings.TrimSpace(c.Query("status")),
	})
	if err != nil {
		response.Error(c, 500, err.Error())
		return
	}
	response.OK(c, data)
}

func (h *QueryHandler) ListRealtimeAlarms(c *gin.Context) {
	page, pageSize := readPageParams(c)
	filter, err := readAlarmListFilter(c)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	data, err := h.queryService.ListRealtimeAlarms(page, pageSize, filter)
	if err != nil {
		response.Error(c, 500, err.Error())
		return
	}
	response.OK(c, data)
}

func (h *QueryHandler) ListAlarms(c *gin.Context) {
	page, pageSize := readPageParams(c)
	filter, err := readAlarmListFilter(c)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	data, err := h.queryService.ListAlarms(page, pageSize, filter)
	if err != nil {
		response.Error(c, 500, err.Error())
		return
	}
	response.OK(c, data)
}

func (h *QueryHandler) DashboardSummary(c *gin.Context) {
	startAt, endAt, err := readOptionalTimeRange(c)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	data, err := h.queryService.GetDashboardSummary(startAt, endAt)
	if err != nil {
		response.Error(c, 500, err.Error())
		return
	}
	response.OK(c, data)
}

func readPageParams(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSizeRaw := c.Query("page_size")
	if pageSizeRaw == "" {
		pageSizeRaw = c.DefaultQuery("pageSize", "20")
	}
	pageSize, _ := strconv.Atoi(pageSizeRaw)
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	return page, pageSize
}

func readOptionalUintQuery(c *gin.Context, key string) (uint, error) {
	value := strings.TrimSpace(c.Query(key))
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, err
	}
	return uint(parsed), nil
}

func readOptionalBoolQuery(c *gin.Context, key string) (*bool, error) {
	value := strings.TrimSpace(c.Query(key))
	if value == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func readAlarmListFilter(c *gin.Context) (dto.AlarmListFilter, error) {
	startAt, endAt, err := readOptionalTimeRange(c)
	if err != nil {
		return dto.AlarmListFilter{}, err
	}
	return dto.AlarmListFilter{
		Keyword:   c.Query("keyword"),
		Status:    c.Query("status"),
		Level:     c.Query("level"),
		AlarmType: c.Query("alarm_type"),
		StartAt:   startAt,
		EndAt:     endAt,
	}, nil
}
