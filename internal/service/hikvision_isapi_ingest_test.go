package service

import (
	"strings"
	"testing"
)

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

func TestParseHikvisionISAPIMotionPayloadMultipartFields(t *testing.T) {
	payload := map[string]any{
		"fields": map[string][]string{
			"body": {
				`<EventNotificationAlert>
					<ipAddress>10.0.0.8</ipAddress>
					<dynChannelID>5</dynChannelID>
					<eventType>motionDetection</eventType>
					<eventState>active</eventState>
				</EventNotificationAlert>`,
			},
		},
	}

	event, ok := parseHikvisionISAPIMotionPayload(payload, map[string]string{})
	if !ok {
		t.Fatalf("expected multipart field motion event")
	}
	if event.DeviceIP != "10.0.0.8" {
		t.Fatalf("unexpected device ip: %s", event.DeviceIP)
	}
	if event.ChannelNo != 5 {
		t.Fatalf("unexpected channel no: %d", event.ChannelNo)
	}
}

func TestParseHikvisionISAPIMotionPayloadFormFields(t *testing.T) {
	payload := map[string]any{
		"fields": map[string][]string{
			"ipAddress":  {"10.0.0.9"},
			"channelID":  {"6"},
			"eventType":  {"VMD"},
			"eventState": {"active"},
		},
	}

	event, ok := parseHikvisionISAPIMotionPayload(payload, map[string]string{})
	if !ok {
		t.Fatalf("expected form field motion event")
	}
	if event.DeviceIP != "10.0.0.9" {
		t.Fatalf("unexpected device ip: %s", event.DeviceIP)
	}
	if event.ChannelNo != 6 {
		t.Fatalf("unexpected channel no: %d", event.ChannelNo)
	}
}

func TestParseHikvisionISAPIMotionPayloadChineseDescription(t *testing.T) {
	payload := `<EventNotificationAlert>
		<ipAddress>192.168.1.65</ipAddress>
		<channelID>2</channelID>
		<eventDescription>移动侦测报警</eventDescription>
		<eventState>active</eventState>
	</EventNotificationAlert>`

	event, ok := parseHikvisionISAPIMotionPayload(payload, map[string]string{})
	if !ok {
		t.Fatalf("expected chinese motion event")
	}
	if event.DeviceIP != "192.168.1.65" {
		t.Fatalf("unexpected device ip: %s", event.DeviceIP)
	}
	if event.ChannelNo != 2 {
		t.Fatalf("unexpected channel no: %d", event.ChannelNo)
	}
}

func TestParseHikvisionISAPIMotionPayloadJSONString(t *testing.T) {
	payload := `{
		"ipAddress": "192.168.1.66",
		"channelID": 7,
		"eventType": "VMD",
		"eventState": "active"
	}`

	event, ok := parseHikvisionISAPIMotionPayload(payload, map[string]string{})
	if !ok {
		t.Fatalf("expected json motion event")
	}
	if event.DeviceIP != "192.168.1.66" {
		t.Fatalf("unexpected device ip: %s", event.DeviceIP)
	}
	if event.ChannelNo != 7 {
		t.Fatalf("unexpected channel no: %d", event.ChannelNo)
	}
}

func TestParseHikvisionISAPIMotionPayloadMultipartJSONField(t *testing.T) {
	payload := map[string]any{
		"fields": map[string][]string{
			"body": {
				`{
					"ipAddress": "10.0.0.10",
					"dynChannelID": 8,
					"eventType": "motionDetection",
					"eventState": "active"
				}`,
			},
		},
	}

	event, ok := parseHikvisionISAPIMotionPayload(payload, map[string]string{})
	if !ok {
		t.Fatalf("expected multipart json field motion event")
	}
	if event.DeviceIP != "10.0.0.10" {
		t.Fatalf("unexpected device ip: %s", event.DeviceIP)
	}
	if event.ChannelNo != 8 {
		t.Fatalf("unexpected channel no: %d", event.ChannelNo)
	}
}

func TestNextISAPITextPayloadWithoutNewline(t *testing.T) {
	var buffer strings.Builder
	buffer.WriteString(`--boundary`)
	buffer.WriteString(`<EventNotificationAlert><ipAddress>10.0.0.11</ipAddress><channelID>9</channelID><eventType>VMD</eventType><eventState>active</eventState></EventNotificationAlert>`)
	buffer.WriteString(`<EventNotificationAlert><ipAddress>10.0.0.12</ipAddress><channelID>10</channelID><eventType>VMD</eventType><eventState>active</eventState></EventNotificationAlert>`)

	first, ok := nextISAPITextPayload(&buffer)
	if !ok {
		t.Fatalf("expected first text payload")
	}
	event, ok := parseHikvisionISAPIMotionPayload(first, map[string]string{})
	if !ok {
		t.Fatalf("expected first motion event")
	}
	if event.ChannelNo != 9 {
		t.Fatalf("unexpected first channel no: %d", event.ChannelNo)
	}

	second, ok := nextISAPITextPayload(&buffer)
	if !ok {
		t.Fatalf("expected second text payload")
	}
	event, ok = parseHikvisionISAPIMotionPayload(second, map[string]string{})
	if !ok {
		t.Fatalf("expected second motion event")
	}
	if event.ChannelNo != 10 {
		t.Fatalf("unexpected second channel no: %d", event.ChannelNo)
	}
}
