//go:build windows

package hikvision

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
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
	sdkPath string
	libDir  string
	dllPath string

	dll                    *syscall.LazyDLL
	procInit               *syscall.LazyProc
	procCleanup            *syscall.LazyProc
	procSetConnectTime     *syscall.LazyProc
	procSetReconnect       *syscall.LazyProc
	procGetLastError       *syscall.LazyProc
	procLoginV40           *syscall.LazyProc
	procLoginV30           *syscall.LazyProc
	procLogout             *syscall.LazyProc
	procSetMessageCallback *syscall.LazyProc
	procSetupAlarm         *syscall.LazyProc
	procCloseAlarm         *syscall.LazyProc
	procCaptureJPEG        *syscall.LazyProc
	procGetFileByTime      *syscall.LazyProc
	procPlayBackControl    *syscall.LazyProc
	procStopGetFile        *syscall.LazyProc

	mu           sync.RWMutex
	handler      AlarmHandler
	callbackPtr  uintptr
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

type userLoginInfo struct {
	DeviceAddress [129]byte
	UseTransport  byte
	Port          uint16
	Username      [64]byte
	Password      [64]byte
	LoginResult   uintptr
	User          uintptr
	UseAsyncLogin int32
	ProxyType     byte
	UseUTCTime    byte
	LoginMode     byte
	HTTPS         byte
	ProxyID       int32
	VerifyMode    byte
	Res3          [119]byte
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
	Res2            [11]byte
}

type setupAlarmParam struct {
	Size               uint32
	Level              byte
	AlarmInfoType      byte
	RetAlarmTypeV40    byte
	RetDevInfoVersion  byte
	RetVQDAlarmType    byte
	FaceAlarmDetection byte
	Support            byte
	BrokenNetHTTP      byte
	TaskNo             uint16
	DeployType         byte
	Res1               [3]byte
	AlarmTypeURL       byte
	CustomCtrl         byte
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

type jpegPara struct {
	PicSize    uint16
	PicQuality uint16
}

type dvrTime struct {
	Year   uint32
	Month  uint32
	Day    uint32
	Hour   uint32
	Minute uint32
	Second uint32
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

func NewSDK(sdkPath string) (*SDK, error) {
	root := strings.TrimSpace(sdkPath)
	if root == "" {
		return nil, fmt.Errorf("hikvision sdk path is empty")
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve hikvision sdk path: %w", err)
	}
	if _, statErr := os.Stat(absRoot); statErr != nil {
		if cwd, cwdErr := os.Getwd(); cwdErr == nil {
			prefix := filepath.Base(filepath.Clean(cwd))
			normalizedRoot := strings.ReplaceAll(root, "/", `\`)
			trimmed := strings.TrimPrefix(normalizedRoot, prefix+`\`)
			if trimmed != normalizedRoot {
				if fallbackRoot, fallbackErr := filepath.Abs(trimmed); fallbackErr == nil {
					if _, fallbackStatErr := os.Stat(fallbackRoot); fallbackStatErr == nil {
						absRoot = fallbackRoot
					}
				}
			}
		}
	}
	libDir := filepath.Join(absRoot, "Lib")
	if _, err := os.Stat(libDir); err != nil {
		libDir = filepath.Join(absRoot, "Library")
	}
	dllPath := filepath.Join(libDir, "HCNetSDK.dll")
	if _, err := os.Stat(dllPath); err != nil {
		return nil, fmt.Errorf("missing HCNetSDK.dll: %w", err)
	}
	return &SDK{
		sdkPath:      absRoot,
		libDir:       libDir,
		dllPath:      dllPath,
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

	_ = os.Setenv("PATH", strings.Join([]string{
		s.libDir,
		filepath.Join(s.libDir, "HCNetSDKCom"),
		s.sdkPath,
		os.Getenv("PATH"),
	}, ";"))

	dll := syscall.NewLazyDLL(s.dllPath)
	procInit := dll.NewProc("NET_DVR_Init")
	procCleanup := dll.NewProc("NET_DVR_Cleanup")
	procSetConnectTime := dll.NewProc("NET_DVR_SetConnectTime")
	procSetReconnect := dll.NewProc("NET_DVR_SetReconnect")
	procGetLastError := dll.NewProc("NET_DVR_GetLastError")
	procLoginV40 := dll.NewProc("NET_DVR_Login_V40")
	procLoginV30 := dll.NewProc("NET_DVR_Login_V30")
	procLogout := dll.NewProc("NET_DVR_Logout")
	procSetMessageCallback := dll.NewProc("NET_DVR_SetDVRMessageCallBack_V31")
	procSetupAlarm := dll.NewProc("NET_DVR_SetupAlarmChan_V41")
	procCloseAlarm := dll.NewProc("NET_DVR_CloseAlarmChan_V30")
	procCaptureJPEG := dll.NewProc("NET_DVR_CaptureJPEGPicture_NEW")
	procGetFileByTime := dll.NewProc("NET_DVR_GetFileByTime")
	procPlayBackControl := dll.NewProc("NET_DVR_PlayBackControl")
	procStopGetFile := dll.NewProc("NET_DVR_StopGetFile")

	if err := dll.Load(); err != nil {
		return fmt.Errorf("load HCNetSDK.dll: %w", err)
	}
	if ret, _, _ := procInit.Call(); ret == 0 {
		return fmt.Errorf("NET_DVR_Init failed, code=%d", s.lastErrorCode(procGetLastError))
	}
	procSetConnectTime.Call(uintptr(3000), uintptr(1))
	procSetReconnect.Call(uintptr(10000), uintptr(1))

	s.dll = dll
	s.procInit = procInit
	s.procCleanup = procCleanup
	s.procSetConnectTime = procSetConnectTime
	s.procSetReconnect = procSetReconnect
	s.procGetLastError = procGetLastError
	s.procLoginV40 = procLoginV40
	s.procLoginV30 = procLoginV30
	s.procLogout = procLogout
	s.procSetMessageCallback = procSetMessageCallback
	s.procSetupAlarm = procSetupAlarm
	s.procCloseAlarm = procCloseAlarm
	s.procCaptureJPEG = procCaptureJPEG
	s.procGetFileByTime = procGetFileByTime
	s.procPlayBackControl = procPlayBackControl
	s.procStopGetFile = procStopGetFile
	if s.sessions == nil {
		s.sessions = make(map[int32]SessionInfo)
	}
	if s.sessionsByIP == nil {
		s.sessionsByIP = make(map[string]SessionInfo)
	}
	s.initialized = true
	return nil
}

func (s *SDK) Cleanup() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.initialized || s.procCleanup == nil {
		return nil
	}
	s.procCleanup.Call()
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
	if s.callbackPtr == 0 {
		s.callbackPtr = syscall.NewCallback(s.messageCallback)
	}
	if ret, _, _ := s.procSetMessageCallback.Call(s.callbackPtr, 0); ret == 0 {
		return fmt.Errorf("NET_DVR_SetDVRMessageCallBack_V31 failed, code=%d", s.lastErrorCode(s.procGetLastError))
	}
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
	loginIP := strings.TrimSpace(ip)
	loginUser := strings.TrimSpace(username)

	if s.procLoginV40 != nil {
		var loginInfo userLoginInfo
		copyCBytes(loginInfo.DeviceAddress[:], loginIP)
		loginInfo.Port = uint16(port)
		copyCBytes(loginInfo.Username[:], loginUser)
		copyCBytes(loginInfo.Password[:], password)
		loginInfo.UseAsyncLogin = 0
		loginInfo.LoginMode = 0
		loginInfo.HTTPS = 0

		var info deviceInfoV40
		ret, _, _ := s.procLoginV40.Call(
			uintptr(unsafe.Pointer(&loginInfo)),
			uintptr(unsafe.Pointer(&info)),
		)
		userID := int32(int(ret))
		if userID >= 0 {
			return userID, DeviceInfo{
				StartChan:  info.DeviceInfoV30.StartChan,
				StartDChan: info.DeviceInfoV30.StartDChan,
			}, nil
		}
	}

	ipPtr, err := syscall.BytePtrFromString(loginIP)
	if err != nil {
		return -1, DeviceInfo{}, err
	}
	userPtr, err := syscall.BytePtrFromString(loginUser)
	if err != nil {
		return -1, DeviceInfo{}, err
	}
	passwordPtr, err := syscall.BytePtrFromString(password)
	if err != nil {
		return -1, DeviceInfo{}, err
	}
	var info deviceInfoV30
	ret, _, _ := s.procLoginV30.Call(
		uintptr(unsafe.Pointer(ipPtr)),
		uintptr(uint16(port)),
		uintptr(unsafe.Pointer(userPtr)),
		uintptr(unsafe.Pointer(passwordPtr)),
		uintptr(unsafe.Pointer(&info)),
	)
	userID := int32(int(ret))
	if userID < 0 {
		return -1, DeviceInfo{}, fmt.Errorf("NET_DVR_Login_V40/V30 failed, code=%d", s.lastErrorCode(s.procGetLastError))
	}
	return userID, DeviceInfo{
		StartChan:  info.StartChan,
		StartDChan: info.StartDChan,
	}, nil
}

func (s *SDK) Logout(userID int32) error {
	if !s.initialized || s.procLogout == nil || userID < 0 {
		return nil
	}
	if ret, _, _ := s.procLogout.Call(uintptr(userID)); ret == 0 {
		return fmt.Errorf("NET_DVR_Logout failed, code=%d", s.lastErrorCode(s.procGetLastError))
	}
	s.UnregisterSession(userID)
	return nil
}

func (s *SDK) SetupMotionAlarm(userID int32) (int32, error) {
	if err := s.Init(); err != nil {
		return -1, err
	}
	param := setupAlarmParam{
		Size:            uint32(unsafe.Sizeof(setupAlarmParam{})),
		Level:           1,
		AlarmInfoType:   1,
		RetAlarmTypeV40: 1,
		DeployType:      1,
	}
	ret, _, _ := s.procSetupAlarm.Call(uintptr(userID), uintptr(unsafe.Pointer(&param)))
	handle := int32(int(ret))
	if handle < 0 {
		return -1, fmt.Errorf("NET_DVR_SetupAlarmChan_V41 failed, code=%d", s.lastErrorCode(s.procGetLastError))
	}
	return handle, nil
}

func (s *SDK) CloseAlarm(alarmHandle int32) error {
	if !s.initialized || s.procCloseAlarm == nil || alarmHandle < 0 {
		return nil
	}
	if ret, _, _ := s.procCloseAlarm.Call(uintptr(alarmHandle)); ret == 0 {
		return fmt.Errorf("NET_DVR_CloseAlarmChan_V30 failed, code=%d", s.lastErrorCode(s.procGetLastError))
	}
	return nil
}

func (s *SDK) CaptureJPEG(userID int32, channelNo int, outputPath string, deviceInfo DeviceInfo) ([]byte, error) {
	if err := s.Init(); err != nil {
		return nil, err
	}
	if s.procCaptureJPEG == nil {
		return nil, fmt.Errorf("NET_DVR_CaptureJPEGPicture_NEW not available")
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
	if s.procGetFileByTime == nil || s.procPlayBackControl == nil || s.procStopGetFile == nil {
		return fmt.Errorf("playback download API not available")
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

	outputPathPtr, err := syscall.BytePtrFromString(outputPath)
	if err != nil {
		return fmt.Errorf("encode playback path: %w", err)
	}

	start := toSDKTime(startTime)
	end := toSDKTime(endTime)
	ret, _, _ := s.procGetFileByTime.Call(
		uintptr(userID),
		uintptr(maxInt(channelNo, 1)),
		uintptr(unsafe.Pointer(&start)),
		uintptr(unsafe.Pointer(&end)),
		uintptr(unsafe.Pointer(outputPathPtr)),
	)
	handle := int32(int(ret))
	if handle < 0 {
		return fmt.Errorf("NET_DVR_GetFileByTime failed, code=%d", s.lastErrorCode(s.procGetLastError))
	}

	stopped := false
	defer func() {
		if !stopped {
			_, stopErr := s.stopGetFile(handle)
			if err == nil && stopErr != nil {
				err = stopErr
			}
		}
		if err != nil {
			_ = os.Remove(outputPath)
		}
	}()

	if ret, _, _ := s.procPlayBackControl.Call(
		uintptr(handle),
		uintptr(netDVRPlayStart),
		0,
		0,
	); ret == 0 {
		return fmt.Errorf("NET_DVR_PlayBackControl(PLAYSTART) failed, code=%d", s.lastErrorCode(s.procGetLastError))
	}

	for {
		var pos uint32
		if ret, _, _ := s.procPlayBackControl.Call(
			uintptr(handle),
			uintptr(netDVRPlayGetPos),
			0,
			uintptr(unsafe.Pointer(&pos)),
		); ret == 0 {
			return fmt.Errorf("NET_DVR_PlayBackControl(PLAYGETPOS) failed, code=%d", s.lastErrorCode(s.procGetLastError))
		}
		if pos == 100 {
			_, stopErr := s.stopGetFile(handle)
			stopped = true
			return stopErr
		}
		if pos > 100 {
			return fmt.Errorf("playback download interrupted, progress=%d", pos)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func (s *SDK) stopGetFile(handle int32) (bool, error) {
	if handle < 0 {
		return true, nil
	}
	if ret, _, _ := s.procStopGetFile.Call(uintptr(handle)); ret == 0 {
		return false, fmt.Errorf("NET_DVR_StopGetFile failed, code=%d", s.lastErrorCode(s.procGetLastError))
	}
	return true, nil
}

func (s *SDK) captureJPEGRaw(userID int32, channelNo int, outputPath string) ([]byte, error) {
	param := jpegPara{
		PicSize:    0xFF,
		PicQuality: 0,
	}
	buffer := make([]byte, 8*1024*1024)
	var returned uint32
	ret, _, _ := s.procCaptureJPEG.Call(
		uintptr(userID),
		uintptr(maxInt(channelNo, 1)),
		uintptr(unsafe.Pointer(&param)),
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(uint32(len(buffer))),
		uintptr(unsafe.Pointer(&returned)),
	)
	if ret == 0 {
		return nil, fmt.Errorf("NET_DVR_CaptureJPEGPicture_NEW failed, code=%d", s.lastErrorCode(s.procGetLastError))
	}
	if returned == 0 {
		return nil, fmt.Errorf("NET_DVR_CaptureJPEGPicture_NEW returned empty data")
	}
	imageBytes := append([]byte(nil), buffer[:returned]...)
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return nil, fmt.Errorf("create snapshot dir: %w", err)
	}
	if err := os.WriteFile(outputPath, imageBytes, 0o644); err != nil {
		return nil, fmt.Errorf("write snapshot file: %w", err)
	}
	return imageBytes, nil
}

func toSDKTime(value time.Time) dvrTime {
	localTime := value.Local()
	return dvrTime{
		Year:   uint32(localTime.Year()),
		Month:  uint32(localTime.Month()),
		Day:    uint32(localTime.Day()),
		Hour:   uint32(localTime.Hour()),
		Minute: uint32(localTime.Minute()),
		Second: uint32(localTime.Second()),
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

func (s *SDK) messageCallback(lCommand, pAlarmer, pAlarmInfo, _dwBufLen, _pUser uintptr) uintptr {
	var deviceIP string
	var userID int32
	if pAlarmer != 0 {
		item := (*alarmer)(unsafe.Pointer(pAlarmer))
		userID = item.UserID
		deviceIP = decodeCString(item.DeviceIP[:])
	}

	s.mu.RLock()
	handler := s.handler
	sessionInfo, ok := s.findSessionLocked(userID, deviceIP)
	s.mu.RUnlock()
	isMotion, channels := parseMotionAlarm(int32(lCommand), pAlarmInfo)
	if !isMotion {
		return 1
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
	if handler != nil {
		go handler(MotionAlarm{
			UserID:   userID,
			Command:  int32(lCommand),
			DeviceIP: deviceIP,
			Channels: channels,
		})
	}
	return 1
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

func (s *SDK) lastErrorCode(proc *syscall.LazyProc) uint32 {
	if proc == nil {
		return 0
	}
	ret, _, _ := proc.Call()
	return uint32(ret)
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

func copyCBytes(dst []byte, value string) {
	if len(dst) == 0 {
		return
	}
	copy(dst[:len(dst)-1], []byte(value))
	dst[len(dst)-1] = 0
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
