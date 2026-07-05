package service

import "testing"

func TestParseHikvisionISAPIMotionPayloadXML(t *testing.T) {
	payload := `<EventNotificationAlert>
		<ipAddress>192.168.1.64</ipAddress>
		<channelID>3</channelID>
		<eventType>VMD</eventType>
		<eventState>active</eventState>
	</EventNotificationAlert>`

	event, ok := parseHikvisionISAPIMotionPayload(payload, map[string]string{})
	if !ok {
		t.Fatalf("expected motion event")
	}
	if event.DeviceIP != "192.168.1.64" {
		t.Fatalf("unexpected device ip: %s", event.DeviceIP)
	}
	if event.ChannelNo != 3 {
		t.Fatalf("unexpected channel no: %d", event.ChannelNo)
	}
}

func TestParseHikvisionISAPIMotionPayloadIgnoresInactive(t *testing.T) {
	payload := map[string]any{
		"ipAddress":  "192.168.1.64",
		"channelID":  float64(1),
		"eventType":  "VMD",
		"eventState": "inactive",
	}

	if _, ok := parseHikvisionISAPIMotionPayload(payload, map[string]string{}); ok {
		t.Fatalf("expected inactive event to be ignored")
	}
}
