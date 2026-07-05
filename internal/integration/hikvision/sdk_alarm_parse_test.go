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

func TestExtractMotionAlarmImageFromISAPI(t *testing.T) {
	image := []byte{0xff, 0xd8, 0x01, 0x02, 0xff, 0xd9}
	pictures := []alarmISAPIPicData{
		{
			PicLen:  uint32(len(image)),
			PicType: alarmISAPIPicTypeJPG,
			PicData: uintptr(unsafe.Pointer(&image[0])),
		},
	}
	info := alarmISAPIInfo{
		PicturesNum: byte(len(pictures)),
		PicPackData: uintptr(unsafe.Pointer(&pictures[0])),
	}

	extracted, imageType := extractMotionAlarmImage(commISAPIAlarm, uintptr(unsafe.Pointer(&info)))
	if imageType != alarmISAPIPicTypeJPG {
		t.Fatalf("unexpected image type: %d", imageType)
	}
	if len(extracted) != len(image) {
		t.Fatalf("unexpected image length: %d", len(extracted))
	}
	for index := range image {
		if extracted[index] != image[index] {
			t.Fatalf("unexpected image byte at %d: %d", index, extracted[index])
		}
	}
	image[2] = 0x99
	if extracted[2] == image[2] {
		t.Fatalf("expected extracted image to be copied from SDK callback memory")
	}
}
