//go:build linux

package hikvision

/*
#cgo CFLAGS: -I${SRCDIR}
#cgo CXXFLAGS: -std=gnu++11 -I${SRCDIR} -I${SRCDIR}/../../../third_party/HCNetSDK_Linux64/Header
#cgo LDFLAGS: -L${SRCDIR}/../../../third_party/HCNetSDK_Linux64/Library -lhcnetsdk -ldl -lstdc++
#include <stdlib.h>
#include "sdk_linux_bridge.h"
*/
import "C"

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unsafe"
)

const (
	commAlarmV30             = 0x4000
	commAlarmV40             = 0x4007
	commISAPIAlarm           = 0x6009
	motionDetectionAlarmType = 3
	netDVRPlayStart          = 1
	netDVRPlayGetPos         = 13
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

type SDK struct {
	sdkPath    string
	libDir     string
	soPath     string
	cryptoPath string
	sslPath    string

	mu           sync.RWMutex
	handler      AlarmHandler
	sessions     map[int32]SessionInfo
	sessionsByIP map[string]SessionInfo
	initialized  bool
}

type deviceInfoV30 struct {
	SerialNumber       [48]byte
	AlarmInPortNum     byte
	AlarmOutPortNum    byte
	DiskNum            byte
	DVRType            byte
	ChanNum            byte
	StartChan          byte
	AudioChanNum       byte
	IPChanNum          byte
	ZeroChanNum        byte
	MainProto          byte
	SubProto           byte
	Support            byte
	Support1           byte
	Support2           byte
	DevType            uint16
	Support3           byte
	MultiStreamProto   byte
	StartDChan         byte
	StartDTalkChan     byte
	HighDChanNum       byte
	Support4           byte
	LanguageType       byte
	VoiceInChanNum     byte
	StartVoiceInChanNo byte
	Support5           byte
	Support6           byte
	MirrorChanNum      byte
	StartMirrorChanNo  uint16
	Support7           byte
	Res2               byte
}

type deviceInfoV40 struct {
	DeviceInfoV30        deviceInfoV30
	SupportLock          byte
	RetryLoginTime       byte
	PasswordLevel        byte
	ProxyType            byte
	SurplusLockTime      uint32
	CharEncodeType       byte
	SupportDev5          byte
	Support              byte
	LoginMode            byte
	OEMCode              uint32
	ResidualValidity     int32
	HasResidualValidity  byte
	SingleStartDTalkChan byte
	SingleDTalkChanNums  byte
	PasswordResetLevel   byte
	SupportStreamEncrypt byte
	MarketType           byte
	TLSCap               byte
	ChildManage          byte
	PlaybackNewPosCap    byte
	Res2                 [235]byte
}

type alarmer struct {
	UserIDValid     byte
	SerialValid     byte
	VersionValid    byte
	DeviceNameValid byte
	MacAddrValid    byte
	LinkPortValid   byte
	DeviceIPValid   byte
	SocketIPValid   byte
	UserID          int32
	SerialNumber    [48]byte
	DeviceVersion   uint32
	DeviceName      [32]byte
	MacAddr         [6]byte
	LinkPort        uint16
	DeviceIP        [128]byte
	SocketIP        [128]byte
	IPProtocol      byte
	Res1            [2]byte
	JSONBroken      byte
	SocketPort      uint16
	Res2            [6]byte
}

type alarmInfoV30 struct {
	AlarmType          uint32
	AlarmInputNumber   uint32
	AlarmOutputNumber  [96]byte
	AlarmRelateChannel [64]byte
	Channel            [64]byte
	DiskNumber         [33]byte
}

type dvrTimeEx struct {
	Year   uint16
	Month  byte
	Day    byte
	Hour   byte
	Minute byte
	Second byte
	Res    byte
}

type alarmFixedHeaderV40 struct {
	AlarmType          uint32
	AlarmTime          dvrTimeEx
	AlarmChannelUnion  [116]byte
	Res                uintptr
	TimeDiffFlag       byte
	TimeDifferenceHour byte
	TimeDifferenceMin  byte
	ResByte            byte
	DevInfoIvmsChannel uint16
	Res2               [2]byte
}

type alarmInfoV40 struct {
	Fixed     alarmFixedHeaderV40
	AlarmData uintptr
}

type alarmISAPIInfo struct {
	AlarmData    uintptr
	AlarmDataLen uint32
	DataType     byte
	PicturesNum  byte
	Res          [2]byte
	PicPackData  uintptr
	Res1         [32]byte
}

var (
	linuxSDKRegistryMu sync.RWMutex
	linuxSDKRegistry   = map[*SDK]struct{}{}
)

func NewSDK(sdkPath string) (*SDK, error) {
	root := strings.TrimSpace(sdkPath)
	if root == "" {
		return nil, fmt.Errorf("hikvision sdk path is empty")
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve hikvision sdk path: %w", err)
	}
	libDir := filepath.Join(absRoot, "Library")
	soPath := filepath.Join(libDir, "libhcnetsdk.so")
	if _, err := os.Stat(soPath); err != nil {
		return nil, fmt.Errorf("missing libhcnetsdk.so: %w", err)
	}
	cryptoPath, err := firstExistingPath(
		filepath.Join(libDir, "libcrypto.so.3"),
		filepath.Join(libDir, "libcrypto.so"),
	)
	if err != nil {
		return nil, err
	}
	sslPath, err := firstExistingPath(
		filepath.Join(libDir, "libssl.so.3"),
		filepath.Join(libDir, "libssl.so"),
	)
	if err != nil {
		return nil, err
	}
	return &SDK{
		sdkPath:      absRoot,
		libDir:       libDir,
		soPath:       soPath,
		cryptoPath:   cryptoPath,
		sslPath:      sslPath,
		sessions:     make(map[int32]SessionInfo),
		sessionsByIP: make(map[string]SessionInfo),
	}, nil
}

func (s *SDK) Init() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.initialized {
		return nil
	}

	_ = os.Setenv("LD_LIBRARY_PATH", strings.Join([]string{
		s.libDir,
		filepath.Join(s.libDir, "HCNetSDKCom"),
		os.Getenv("LD_LIBRARY_PATH"),
	}, ":"))

	sdkPath := C.CString(sdkInitDir(s.libDir))
	cryptoPath := C.CString(s.cryptoPath)
	sslPath := C.CString(s.sslPath)
	defer C.free(unsafe.Pointer(sdkPath))
	defer C.free(unsafe.Pointer(cryptoPath))
	defer C.free(unsafe.Pointer(sslPath))

	if C.hik_setup_sdk_init_paths(sdkPath, cryptoPath, sslPath) == 0 {
		return fmt.Errorf("NET_DVR_SetSDKInitCfg failed, code=%d", s.lastErrorCode())
	}
	if C.hik_sdk_init() == 0 {
		return fmt.Errorf("NET_DVR_Init failed, code=%d", s.lastErrorCode())
	}
	C.hik_set_connect_time(3000, 1)
	C.hik_set_reconnect(10000, 1)
	if C.hik_setup_sdk_local_config() == 0 {
		C.hik_sdk_cleanup()
		return fmt.Errorf("NET_DVR_SetSDKLocalCfg failed, code=%d", s.lastErrorCode())
	}

	registerLinuxSDK(s)
	s.initialized = true
	return nil
}

func (s *SDK) Cleanup() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.initialized {
		return nil
	}
	unregisterLinuxSDK(s)
	C.hik_sdk_cleanup()
	s.initialized = false
	s.sessions = make(map[int32]SessionInfo)
	s.sessionsByIP = make(map[string]SessionInfo)
	return nil
}

func (s *SDK) SetAlarmHandler(handler AlarmHandler) error {
	if err := s.Init(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handler = handler
	if C.hik_set_alarm_callback() == 0 {
		return fmt.Errorf("NET_DVR_SetDVRMessageCallBack_V31 failed, code=%d", s.lastErrorCode())
	}
	debugAlarmf("alarm callback registered")
	return nil
}

func (s *SDK) LoginCamera(ip string, port int, username, password string) (int32, DeviceInfo, error) {
	return s.login(ip, port, username, password)
}

func (s *SDK) LoginRecorder(ip string, port int, username, password string) (int32, DeviceInfo, error) {
	return s.login(ip, port, username, password)
}

func (s *SDK) login(ip string, port int, username, password string) (int32, DeviceInfo, error) {
	if err := s.Init(); err != nil {
		return -1, DeviceInfo{}, err
	}

	loginIP := C.CString(strings.TrimSpace(ip))
	loginUser := C.CString(strings.TrimSpace(username))
	loginPassword := C.CString(password)
	defer C.free(unsafe.Pointer(loginIP))
	defer C.free(unsafe.Pointer(loginUser))
	defer C.free(unsafe.Pointer(loginPassword))

	var info C.HikDeviceInfo
	userID := int32(C.hik_login_v40(loginIP, C.ushort(uint16(port)), loginUser, loginPassword, &info))
	if userID < 0 {
		return -1, DeviceInfo{}, fmt.Errorf("NET_DVR_Login_V40 failed, code=%d", s.lastErrorCode())
	}
	return userID, DeviceInfo{
		StartChan:  byte(info.startChan),
		StartDChan: byte(info.startDChan),
	}, nil
}

func (s *SDK) Logout(userID int32) error {
	if userID < 0 {
		return nil
	}
	if C.hik_logout(C.int(userID)) == 0 {
		return fmt.Errorf("NET_DVR_Logout failed, code=%d", s.lastErrorCode())
	}
	s.UnregisterSession(userID)
	return nil
}

func (s *SDK) SetupMotionAlarm(userID int32) (int32, error) {
	if err := s.Init(); err != nil {
		return -1, err
	}
	handle := int32(C.hik_setup_alarm_chan_v41(C.int(userID)))
	if handle < 0 {
		return -1, fmt.Errorf("NET_DVR_SetupAlarmChan_V41 failed, code=%d", s.lastErrorCode())
	}
	debugAlarmf("alarm channel setup succeeded userID=%d handle=%d", userID, handle)
	return handle, nil
}

func (s *SDK) CloseAlarm(alarmHandle int32) error {
	if alarmHandle < 0 {
		return nil
	}
	if C.hik_close_alarm_chan_v30(C.int(alarmHandle)) == 0 {
		return fmt.Errorf("NET_DVR_CloseAlarmChan_V30 failed, code=%d", s.lastErrorCode())
	}
	return nil
}

func (s *SDK) CaptureJPEG(userID int32, channelNo int, outputPath string, deviceInfo DeviceInfo) ([]byte, error) {
	if err := s.Init(); err != nil {
		return nil, err
	}
	var lastErr error
	for _, sdkChannel := range buildCaptureChannelCandidates(deviceInfo, channelNo) {
		data, err := s.captureJPEGRaw(userID, sdkChannel, outputPath)
		if err == nil {
			return data, nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("capture jpeg failed: no channel candidates")
}

func (s *SDK) DownloadRecordByTime(userID int32, channelNo int, startTime, endTime time.Time, outputPath string, deviceInfo DeviceInfo) error {
	if err := s.Init(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("create playback download dir: %w", err)
	}

	var lastErr error
	for _, sdkChannel := range buildCaptureChannelCandidates(deviceInfo, channelNo) {
		if err := s.downloadRecordByTimeRaw(userID, sdkChannel, startTime, endTime, outputPath); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("download playback failed: no channel candidates")
}

func (s *SDK) downloadRecordByTimeRaw(userID int32, channelNo int, startTime, endTime time.Time, outputPath string) (err error) {
	if err := os.Remove(outputPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("cleanup existing playback file: %w", err)
	}

	outputPathC := C.CString(outputPath)
	defer C.free(unsafe.Pointer(outputPathC))

	start := toLinuxSDKTime(startTime)
	end := toLinuxSDKTime(endTime)
	handle := int32(C.hik_get_file_by_time(C.int(userID), C.int(maxInt(channelNo, 1)), &start, &end, outputPathC))
	if handle < 0 {
		return fmt.Errorf("NET_DVR_GetFileByTime failed, code=%d", s.lastErrorCode())
	}

	stopped := false
	defer func() {
		if !stopped {
			if stopErr := s.stopGetFile(handle); err == nil && stopErr != nil {
				err = stopErr
			}
		}
		if err != nil {
			_ = os.Remove(outputPath)
		}
	}()

	if C.hik_playback_control(C.int(handle), C.uint(netDVRPlayStart), 0, nil) == 0 {
		return fmt.Errorf("NET_DVR_PlayBackControl(PLAYSTART) failed, code=%d", s.lastErrorCode())
	}

	for {
		var pos C.uint
		if C.hik_playback_control(C.int(handle), C.uint(netDVRPlayGetPos), 0, &pos) == 0 {
			return fmt.Errorf("NET_DVR_PlayBackControl(PLAYGETPOS) failed, code=%d", s.lastErrorCode())
		}
		progress := uint32(pos)
		if progress == 100 {
			if stopErr := s.stopGetFile(handle); stopErr != nil {
				return stopErr
			}
			stopped = true
			return nil
		}
		if progress > 100 {
			return fmt.Errorf("playback download interrupted, progress=%d", progress)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func (s *SDK) captureJPEGRaw(userID int32, channelNo int, outputPath string) ([]byte, error) {
	bufferSize := 8 * 1024 * 1024
	buffer := C.malloc(C.size_t(bufferSize))
	if buffer == nil {
		return nil, fmt.Errorf("allocate jpeg buffer failed")
	}
	defer C.free(buffer)

	var returned C.uint
	if C.hik_capture_jpeg_new(
		C.int(userID),
		C.int(maxInt(channelNo, 1)),
		(*C.char)(buffer),
		C.uint(bufferSize),
		&returned,
	) == 0 {
		return nil, fmt.Errorf("NET_DVR_CaptureJPEGPicture_NEW failed, code=%d", s.lastErrorCode())
	}
	if returned == 0 {
		return nil, fmt.Errorf("NET_DVR_CaptureJPEGPicture_NEW returned empty data")
	}
	imageBytes := C.GoBytes(buffer, C.int(returned))
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return nil, fmt.Errorf("create snapshot dir: %w", err)
	}
	if err := os.WriteFile(outputPath, imageBytes, 0o644); err != nil {
		return nil, fmt.Errorf("write snapshot file: %w", err)
	}
	return imageBytes, nil
}

func (s *SDK) stopGetFile(handle int32) error {
	if handle < 0 {
		return nil
	}
	if C.hik_stop_get_file(C.int(handle)) == 0 {
		return fmt.Errorf("NET_DVR_StopGetFile failed, code=%d", s.lastErrorCode())
	}
	return nil
}

func toLinuxSDKTime(value time.Time) C.HikTime {
	localTime := value.Local()
	return C.HikTime{
		year:   C.uint(localTime.Year()),
		month:  C.uint(localTime.Month()),
		day:    C.uint(localTime.Day()),
		hour:   C.uint(localTime.Hour()),
		minute: C.uint(localTime.Minute()),
		second: C.uint(localTime.Second()),
	}
}

func (s *SDK) RegisterSession(info SessionInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[info.UserID] = info
	if ip := normalizeSessionIP(info.DeviceIP); ip != "" {
		s.sessionsByIP[ip] = info
	}
}

func (s *SDK) UnregisterSession(userID int32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if info, ok := s.sessions[userID]; ok {
		if ip := normalizeSessionIP(info.DeviceIP); ip != "" {
			delete(s.sessionsByIP, ip)
		}
	}
	delete(s.sessions, userID)
}

func (s *SDK) messageCallback(command int32, pAlarmer unsafe.Pointer, pAlarmInfo unsafe.Pointer, _bufLen uint32) bool {
	var deviceIP string
	var userID int32
	if pAlarmer != nil {
		item := (*alarmer)(pAlarmer)
		userID = item.UserID
		deviceIP = decodeCString(item.DeviceIP[:])
	}
	debugAlarmf("raw callback command=0x%x userID=%d deviceIP=%q hasAlarmInfo=%t", command, userID, deviceIP, pAlarmInfo != nil)

	s.mu.RLock()
	handler := s.handler
	sessionInfo, ok := s.findSessionLocked(userID, deviceIP)
	s.mu.RUnlock()

	isMotion, channels := parseMotionAlarm(command, uintptr(pAlarmInfo))
	if !isMotion {
		debugAlarmf("ignored callback command=0x%x userID=%d deviceIP=%q sessionMatched=%t", command, userID, deviceIP, ok)
		return true
	}
	if ok {
		if deviceIP == "" {
			deviceIP = sessionInfo.DeviceIP
		}
		userID = sessionInfo.UserID
	}
	if len(channels) == 0 {
		channels = []int{1}
	}
	debugAlarmf("motion callback command=0x%x userID=%d deviceIP=%q sessionMatched=%t channels=%v", command, userID, deviceIP, ok, channels)
	if handler != nil {
		go handler(MotionAlarm{
			UserID:   userID,
			Command:  command,
			DeviceIP: deviceIP,
			Channels: channels,
		})
	}
	return true
}

func (s *SDK) findSessionLocked(userID int32, deviceIP string) (SessionInfo, bool) {
	if userID > 0 {
		if info, ok := s.sessions[userID]; ok {
			return info, true
		}
	}
	if ip := normalizeSessionIP(deviceIP); ip != "" {
		if info, ok := s.sessionsByIP[ip]; ok {
			return info, true
		}
	}
	if len(s.sessions) == 1 {
		for _, info := range s.sessions {
			return info, true
		}
	}
	return SessionInfo{}, false
}

func (s *SDK) lastErrorCode() uint32 {
	return uint32(C.hik_get_last_error())
}

func registerLinuxSDK(s *SDK) {
	linuxSDKRegistryMu.Lock()
	defer linuxSDKRegistryMu.Unlock()
	linuxSDKRegistry[s] = struct{}{}
}

func unregisterLinuxSDK(s *SDK) {
	linuxSDKRegistryMu.Lock()
	defer linuxSDKRegistryMu.Unlock()
	delete(linuxSDKRegistry, s)
}

func dispatchLinuxAlarmCallback(command int32, pAlarmer unsafe.Pointer, pAlarmInfo unsafe.Pointer, bufLen uint32) bool {
	linuxSDKRegistryMu.RLock()
	defer linuxSDKRegistryMu.RUnlock()
	for sdk := range linuxSDKRegistry {
		if sdk.messageCallback(command, pAlarmer, pAlarmInfo, bufLen) {
			return true
		}
	}
	return true
}

//export goLinuxHikAlarmCallback
func goLinuxHikAlarmCallback(command C.int, pAlarmer unsafe.Pointer, pAlarmInfo *C.char, dwBufLen C.uint, pUser unsafe.Pointer) C.int {
	_ = pUser
	if dispatchLinuxAlarmCallback(int32(command), pAlarmer, unsafe.Pointer(pAlarmInfo), uint32(dwBufLen)) {
		return 1
	}
	return 0
}

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

func buildCaptureChannelCandidates(deviceInfo DeviceInfo, channelNo int) []int {
	candidates := make([]int, 0, 3)
	if deviceInfo.StartDChan > 0 && channelNo < int(deviceInfo.StartDChan) {
		candidates = append(candidates, int(deviceInfo.StartDChan)+channelNo-1)
	}
	candidates = append(candidates, channelNo)
	if deviceInfo.StartChan > 0 && channelNo < int(deviceInfo.StartChan) {
		candidates = append(candidates, int(deviceInfo.StartChan)+channelNo-1)
	}
	ordered := make([]int, 0, len(candidates))
	for _, item := range candidates {
		if item > 0 && !containsInt(ordered, item) {
			ordered = append(ordered, item)
		}
	}
	return ordered
}

func parseMotionAlarm(command int32, pAlarmInfo uintptr) (bool, []int) {
	if isMotion, channels := parseMotionAlarmV30(command, pAlarmInfo); isMotion {
		return true, channels
	}
	if isMotion, channels := parseMotionAlarmV40(command, pAlarmInfo); isMotion {
		return true, channels
	}
	return parseMotionAlarmISAPI(command, pAlarmInfo)
}

func parseMotionAlarmV30(command int32, pAlarmInfo uintptr) (bool, []int) {
	if command != commAlarmV30 || pAlarmInfo == 0 {
		return false, nil
	}
	info := (*alarmInfoV30)(unsafe.Pointer(pAlarmInfo))
	if int(info.AlarmType) != motionDetectionAlarmType {
		return false, nil
	}
	channels := make([]int, 0, len(info.Channel))
	for index, value := range info.Channel {
		if value == 1 {
			channels = append(channels, index+1)
		}
	}
	return true, channels
}

func parseMotionAlarmV40(command int32, pAlarmInfo uintptr) (bool, []int) {
	if command != commAlarmV40 || pAlarmInfo == 0 {
		return false, nil
	}
	info := (*alarmInfoV40)(unsafe.Pointer(pAlarmInfo))
	if int(info.Fixed.AlarmType) != motionDetectionAlarmType {
		return false, nil
	}
	channelCount := int(uint32(info.Fixed.AlarmChannelUnion[0]) |
		uint32(info.Fixed.AlarmChannelUnion[1])<<8 |
		uint32(info.Fixed.AlarmChannelUnion[2])<<16 |
		uint32(info.Fixed.AlarmChannelUnion[3])<<24)
	if channelCount <= 0 || info.AlarmData == 0 {
		return true, nil
	}
	if channelCount > 512 {
		channelCount = 512
	}
	channelValues := unsafe.Slice((*uint32)(unsafe.Pointer(info.AlarmData)), channelCount)
	channels := make([]int, 0, len(channelValues))
	for _, value := range channelValues {
		if value == 0 || value == 0xffffffff {
			continue
		}
		channel := int(value)
		if !containsInt(channels, channel) {
			channels = append(channels, channel)
		}
	}
	return true, channels
}

func parseMotionAlarmISAPI(command int32, pAlarmInfo uintptr) (bool, []int) {
	if command != commISAPIAlarm || pAlarmInfo == 0 {
		return false, nil
	}
	info := (*alarmISAPIInfo)(unsafe.Pointer(pAlarmInfo))
	if info.AlarmData == 0 || info.AlarmDataLen == 0 {
		return false, nil
	}
	payload := unsafe.Slice((*byte)(unsafe.Pointer(info.AlarmData)), info.AlarmDataLen)
	return parseMotionAlarmPayload(payload)
}

func parseMotionAlarmPayload(payload []byte) (bool, []int) {
	text := strings.TrimSpace(string(payload))
	if text == "" {
		return false, nil
	}

	if strings.HasPrefix(text, "{") || strings.HasPrefix(text, "[") {
		var parsed any
		if err := json.Unmarshal([]byte(text), &parsed); err == nil {
			return jsonContainsMotionEvent(parsed), extractChannelNumbersFromJSON(parsed)
		}
	}

	var root xmlNode
	if err := xml.Unmarshal([]byte(text), &root); err == nil {
		return xmlContainsMotionEvent(root), extractChannelNumbersFromXML(root)
	}

	return textContainsMotionEvent(text), extractChannelNumbersFromText(text)
}

type xmlNode struct {
	XMLName xml.Name
	Content string    `xml:",chardata"`
	Nodes   []xmlNode `xml:",any"`
}

func jsonContainsMotionEvent(value any) bool {
	switch item := value.(type) {
	case map[string]any:
		for key, child := range item {
			keyName := strings.ToLower(strings.TrimSpace(key))
			if isMotionEventField(keyName) {
				if text, ok := child.(string); ok && isMotionEventValue(text) {
					return true
				}
			}
			if jsonContainsMotionEvent(child) {
				return true
			}
		}
	case []any:
		for _, child := range item {
			if jsonContainsMotionEvent(child) {
				return true
			}
		}
	case string:
		return isMotionEventValue(item)
	}
	return false
}

func extractChannelNumbersFromJSON(value any) []int {
	channels := make([]int, 0, 4)
	var visit func(any)
	visit = func(node any) {
		switch item := node.(type) {
		case map[string]any:
			for key, child := range item {
				keyName := strings.ToLower(strings.TrimSpace(key))
				if isChannelField(keyName) {
					if number, ok := coerceChannelNumber(child); ok && !containsInt(channels, number) {
						channels = append(channels, number)
					}
				}
				visit(child)
			}
		case []any:
			for _, child := range item {
				visit(child)
			}
		}
	}
	visit(value)
	return channels
}

func xmlContainsMotionEvent(root xmlNode) bool {
	tag := strings.ToLower(tagName(root.XMLName.Local))
	if isMotionEventField(tag) && isMotionEventValue(root.Content) {
		return true
	}
	for _, child := range root.Nodes {
		if xmlContainsMotionEvent(child) {
			return true
		}
	}
	return false
}

func extractChannelNumbersFromXML(root xmlNode) []int {
	channels := make([]int, 0, 4)
	var visit func(xmlNode)
	visit = func(node xmlNode) {
		tag := strings.ToLower(tagName(node.XMLName.Local))
		if isChannelField(tag) {
			if number, ok := coerceChannelNumber(node.Content); ok && !containsInt(channels, number) {
				channels = append(channels, number)
			}
		}
		for _, child := range node.Nodes {
			visit(child)
		}
	}
	visit(root)
	return channels
}

func textContainsMotionEvent(text string) bool {
	normalized := normalizeMotionText(text)
	for _, keyword := range []string{
		"eventtypevmd",
		"eventtypemotion",
		"eventtypemotiondetection",
		"eventtypemovingdetection",
		"eventdescriptionmotion",
		"eventnamemotion",
	} {
		if strings.Contains(normalized, keyword) {
			return true
		}
	}
	return false
}

func extractChannelNumbersFromText(text string) []int {
	channels := make([]int, 0, 4)
	for _, pattern := range []*regexp.Regexp{
		regexp.MustCompile(`(?i)"channelID"\s*:\s*"?(?P<channel>\d+)"?`),
		regexp.MustCompile(`(?i)<channelID>\s*(?P<channel>\d+)\s*</channelID>`),
		regexp.MustCompile(`(?i)"dynChannelID"\s*:\s*"?(?P<channel>\d+)"?`),
		regexp.MustCompile(`(?i)<dynChannelID>\s*(?P<channel>\d+)\s*</dynChannelID>`),
	} {
		matches := pattern.FindAllStringSubmatch(text, -1)
		index := pattern.SubexpIndex("channel")
		for _, match := range matches {
			if index > 0 && index < len(match) {
				if number, ok := coerceChannelNumber(match[index]); ok && !containsInt(channels, number) {
					channels = append(channels, number)
				}
			}
		}
	}
	return channels
}

func normalizeMotionText(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	builder := strings.Builder{}
	builder.Grow(len(value))
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') {
			builder.WriteRune(char)
		}
	}
	return builder.String()
}

func isMotionEventValue(value string) bool {
	switch normalizeMotionText(value) {
	case "vmd", "motion", "motiondetection", "movingdetection", "movedetection", "videomotion", "motionalarm":
		return true
	default:
		return false
	}
}

func isMotionEventField(value string) bool {
	switch value {
	case "eventtype", "eventname", "eventdescription", "event", "description":
		return true
	default:
		return false
	}
}

func isChannelField(value string) bool {
	switch value {
	case "channelid", "channelno", "dynchannelid", "videoinputchannelid", "channel":
		return true
	default:
		return false
	}
}

func coerceChannelNumber(value any) (int, bool) {
	switch item := value.(type) {
	case float64:
		if int(item) > 0 {
			return int(item), true
		}
	case int:
		if item > 0 {
			return item, true
		}
	case string:
		item = strings.TrimSpace(item)
		if item == "" {
			return 0, false
		}
		var number int
		_, err := fmt.Sscanf(item, "%d", &number)
		if err == nil && number > 0 {
			return number, true
		}
	}
	return 0, false
}

func decodeCString(data []byte) string {
	text := string(data)
	if index := strings.IndexByte(text, 0); index >= 0 {
		text = text[:index]
	}
	return strings.TrimSpace(text)
}

func normalizeSessionIP(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func sdkInitDir(dir string) string {
	cleaned := filepath.Clean(strings.TrimSpace(dir))
	if cleaned == "." {
		return ""
	}
	if strings.HasSuffix(cleaned, string(os.PathSeparator)) {
		return cleaned
	}
	return cleaned + string(os.PathSeparator)
}

func tagName(value string) string {
	if index := strings.Index(value, ":"); index >= 0 {
		return value[index+1:]
	}
	return value
}

func containsInt(items []int, target int) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}

func debugAlarmf(format string, args ...any) {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("HIKVISION_ALARM_DEBUG"))) {
	case "1", "true", "yes", "on":
		log.Printf("[hikvision-alarm-debug] "+format, args...)
	}
}

func firstExistingPath(paths ...string) (string, error) {
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("missing required sdk dependency: %s", strings.Join(paths, ", "))
}
