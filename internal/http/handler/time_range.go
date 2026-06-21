package handler

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
)

const localDateTimeLayout = "2006-01-02T15:04:05"

func readOptionalTimeRange(c *gin.Context) (*time.Time, *time.Time, error) {
	startAt, err := parseOptionalTime(c.Query("start_at"))
	if err != nil {
		return nil, nil, fmt.Errorf("invalid start_at: %w", err)
	}
	endAt, err := parseOptionalTime(c.Query("end_at"))
	if err != nil {
		return nil, nil, fmt.Errorf("invalid end_at: %w", err)
	}
	return startAt, endAt, nil
}

func parseOptionalTime(raw string) (*time.Time, error) {
	if raw == "" {
		return nil, nil
	}
	if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
		return &parsed, nil
	}
	parsed, err := time.ParseInLocation(localDateTimeLayout, raw, time.Local)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}
