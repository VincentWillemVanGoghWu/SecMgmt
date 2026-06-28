//go:build !windows && !linux

package hikvision

import (
	"fmt"
	"time"
)

type DeviceInfo struct {
	StartChan  byte
	StartDChan byte
}

type SessionInfo struct {
	UserID     int32
	SessionKey string
	DeviceType string
	DeviceID   uint
	DeviceName string
	DeviceIP   string
	DeviceInfo DeviceInfo
}

type MotionAlarm struct {
	UserID   int32
	Command  int32
	DeviceIP string
	Channels []int
}

type AlarmHandler func(MotionAlarm)

type SDK struct{}

func NewSDK(_ string) (*SDK, error) {
	return nil, fmt.Errorf("hikvision sdk motion bridge only supports windows runtime")
}

func (s *SDK) Init() error                          { return fmt.Errorf("unsupported platform") }
func (s *SDK) Cleanup() error                       { return nil }
func (s *SDK) SetAlarmHandler(_ AlarmHandler) error { return fmt.Errorf("unsupported platform") }
func (s *SDK) LoginCamera(_ string, _ int, _ string, _ string) (int32, DeviceInfo, error) {
	return -1, DeviceInfo{}, fmt.Errorf("unsupported platform")
}
func (s *SDK) LoginRecorder(_ string, _ int, _ string, _ string) (int32, DeviceInfo, error) {
	return -1, DeviceInfo{}, fmt.Errorf("unsupported platform")
}
func (s *SDK) Logout(_ int32) error                    { return nil }
func (s *SDK) SetupMotionAlarm(_ int32) (int32, error) { return -1, fmt.Errorf("unsupported platform") }
func (s *SDK) CloseAlarm(_ int32) error                { return nil }
func (s *SDK) DownloadRecordByTime(_ int32, _ int, _ time.Time, _ time.Time, _ string, _ DeviceInfo) error {
	return fmt.Errorf("unsupported platform")
}
func (s *SDK) RegisterSession(_ SessionInfo) {}
func (s *SDK) UnregisterSession(_ int32)     {}

func NormalizeAlarmChannelNo(deviceInfo DeviceInfo, channelNo int) int {
	normalized := channelNo
	if normalized < 1 {
		normalized = 1
	}
	if deviceInfo.StartDChan > 0 && normalized >= int(deviceInfo.StartDChan) {
		return normalized - int(deviceInfo.StartDChan) + 1
	}
	return normalized
}
