package handler

import (
	"net/http"
	"strings"

	"secmgmt_go/internal/http/middleware"
	"secmgmt_go/internal/http/response"
	"secmgmt_go/internal/service"

	"github.com/gin-gonic/gin"
)

type OperationLogHandler struct {
	service *service.OperationLogService
}

func NewOperationLogHandler(service *service.OperationLogService) *OperationLogHandler {
	return &OperationLogHandler{service: service}
}

func (h *OperationLogHandler) Track(c *gin.Context) {
	var payload service.OperationLogTrackPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	userID := middleware.CurrentUserID(c)
	actor := service.OperationLogActor{
		UserID:    &userID,
		Username:  middleware.CurrentUsername(c),
		RealName:  middleware.CurrentUserRealName(c),
		RoleCodes: middleware.CurrentRoleCodes(c),
		RoleNames: middleware.CurrentRoleNames(c),
	}
	if userID == 0 {
		actor.UserID = nil
	}
	meta := service.OperationLogCreateInput{
		TraceID:        c.GetHeader("X-Trace-Id"),
		Source:         "ui",
		ClientIP:       c.ClientIP(),
		IPLocation:     "局域网",
		UserAgent:      c.Request.UserAgent(),
		OSName:         c.GetHeader("X-Client-OS"),
		RequestMethod:  c.Request.Method,
		RequestPath:    c.Request.URL.Path,
		RequestQuery:   c.Request.URL.RawQuery,
		ResponseStatus: http.StatusOK,
		ResultStatus:   "success",
	}
	if err := h.service.RecordTrack(actor, meta, payload); err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, gin.H{})
}

func (h *OperationLogHandler) List(c *gin.Context) {
	page, pageSize := readPageParams(c)
	startAt, endAt, err := readOptionalTimeRange(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	data, err := h.service.List(page, pageSize, service.OperationLogListFilter{
		Username:      strings.TrimSpace(c.Query("username")),
		OperationType: strings.TrimSpace(c.Query("operation_type")),
		ResultStatus:  strings.TrimSpace(c.Query("result_status")),
		MenuCode:      strings.TrimSpace(c.Query("menu_code")),
		Keyword:       strings.TrimSpace(c.Query("keyword")),
		StartAt:       startAt,
		EndAt:         endAt,
	})
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, data)
}

func (h *OperationLogHandler) Detail(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	data, err := h.service.GetDetail(id)
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *OperationLogHandler) Export(c *gin.Context) {
	startAt, endAt, err := readOptionalTimeRange(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	content, filename, err := h.service.Export(service.OperationLogListFilter{
		Username:      strings.TrimSpace(c.Query("username")),
		OperationType: strings.TrimSpace(c.Query("operation_type")),
		ResultStatus:  strings.TrimSpace(c.Query("result_status")),
		MenuCode:      strings.TrimSpace(c.Query("menu_code")),
		Keyword:       strings.TrimSpace(c.Query("keyword")),
		StartAt:       startAt,
		EndAt:         endAt,
	})
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	respondAttachment(c, content, filename, "text/csv; charset=utf-8")
}

func (h *OperationLogHandler) DashboardStats(c *gin.Context) {
	startAt, endAt, err := readOptionalTimeRange(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.OK(c, h.service.GetDashboardStats(startAt, endAt))
}
