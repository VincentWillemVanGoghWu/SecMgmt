package service

import "testing"

func TestCompleteRecorderSyncChannelsFillsConfiguredCount(t *testing.T) {
	channels := completeRecorderSyncChannels([]recorderSyncChannel{{
		ChannelNo:    1,
		Name:         "Front Gate",
		Enabled:      true,
		Status:       "online",
		NamePriority: 40,
	}}, 4)

	if len(channels) != 4 {
		t.Fatalf("expected 4 channels, got %d", len(channels))
	}
	for index, channel := range channels {
		expectedNo := index + 1
		if channel.ChannelNo != expectedNo {
			t.Fatalf("expected channel no %d at index %d, got %d", expectedNo, index, channel.ChannelNo)
		}
		if !channel.Enabled {
			t.Fatalf("expected channel %d to be enabled", channel.ChannelNo)
		}
	}
	if channels[0].Name != "Front Gate" {
		t.Fatalf("expected fetched channel name to be preserved, got %q", channels[0].Name)
	}
	if channels[1].Name == "" {
		t.Fatalf("expected missing channel to get a default name")
	}
}

func TestParseHikRecorderChannelsReadsNestedDigitalStatus(t *testing.T) {
	body := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<InputProxyChannelStatusList>
  <InputProxyChannelStatus>
    <id>1</id>
    <sourceInputPortDescriptor>
      <ipAddress>192.168.1.10</ipAddress>
      <online>true</online>
    </sourceInputPortDescriptor>
  </InputProxyChannelStatus>
  <InputProxyChannelStatus>
    <id>2</id>
    <deviceName>Warehouse</deviceName>
    <online>false</online>
  </InputProxyChannelStatus>
</InputProxyChannelStatusList>`)

	channels, err := parseHikRecorderChannels(body, "digital")
	if err != nil {
		t.Fatalf("parseHikRecorderChannels returned error: %v", err)
	}
	if len(channels) != 2 {
		t.Fatalf("expected 2 channels, got %d", len(channels))
	}
	if channels[0].ChannelNo != 1 || channels[0].Name != "192.168.1.10" || channels[0].Status != "online" {
		t.Fatalf("unexpected first channel: %#v", channels[0])
	}
	if channels[1].ChannelNo != 2 || channels[1].Name != "Warehouse" || channels[1].Status != "offline" {
		t.Fatalf("unexpected second channel: %#v", channels[1])
	}
}
