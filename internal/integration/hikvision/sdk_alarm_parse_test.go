package hikvision

import (
	"testing"
	"unsafe"
)

func TestParseMotionAlarmV40(t *testing.T) {
	channels := []uint32{33, 34, 0xffffffff, 34}
	info := alarmInfoV40{
		Fixed: alarmFixedHeaderV40{
			AlarmType: motionDetectionAlarmType,
		},
		AlarmData: uintptr(unsafe.Pointer(&channels[0])),
	}
	info.Fixed.AlarmChannelUnion[0] = byte(len(channels))

	isMotion, parsedChannels := parseMotionAlarm(commAlarmV40, uintptr(unsafe.Pointer(&info)))
	if !isMotion {
		t.Fatalf("expected V40 motion alarm")
	}
	if len(parsedChannels) != 2 || parsedChannels[0] != 33 || parsedChannels[1] != 34 {
		t.Fatalf("unexpected channels: %#v", parsedChannels)
	}
}

func TestParseMotionAlarmV40IgnoresNonMotion(t *testing.T) {
	channels := []uint32{1}
	info := alarmInfoV40{
		Fixed: alarmFixedHeaderV40{
			AlarmType: 2,
		},
		AlarmData: uintptr(unsafe.Pointer(&channels[0])),
	}
	info.Fixed.AlarmChannelUnion[0] = byte(len(channels))

	isMotion, parsedChannels := parseMotionAlarm(commAlarmV40, uintptr(unsafe.Pointer(&info)))
	if isMotion {
		t.Fatalf("expected non-motion V40 alarm to be ignored, channels=%#v", parsedChannels)
	}
}
