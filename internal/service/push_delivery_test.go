package service

import (
	"testing"

	"secmgmt_go/internal/config"
)

func TestResolveEmailSenderConfigUsesDefaults(t *testing.T) {
	host, port, username, password, fromAddress, fromName, err := resolveEmailSenderConfig(&config.Config{})
	if err != nil {
		t.Fatalf("resolveEmailSenderConfig returned error: %v", err)
	}
	if host != config.DefaultPushEmailSMTPHost {
		t.Fatalf("host = %q, want %q", host, config.DefaultPushEmailSMTPHost)
	}
	if port != config.DefaultPushEmailSMTPPort {
		t.Fatalf("port = %d, want %d", port, config.DefaultPushEmailSMTPPort)
	}
	if username != config.DefaultPushEmailUsername {
		t.Fatalf("username = %q, want %q", username, config.DefaultPushEmailUsername)
	}
	if password != config.DefaultPushEmailPassword {
		t.Fatalf("password = %q, want %q", password, config.DefaultPushEmailPassword)
	}
	if fromAddress != config.DefaultPushEmailFrom {
		t.Fatalf("fromAddress = %q, want %q", fromAddress, config.DefaultPushEmailFrom)
	}
	if fromName != config.DefaultPushEmailFromName {
		t.Fatalf("fromName = %q, want %q", fromName, config.DefaultPushEmailFromName)
	}
}

func TestPushChannelAllowedNormalizesEmailAliases(t *testing.T) {
	allowed := []string{"邮件", "wechat"}
	if !pushChannelAllowed(allowed, "email") {
		t.Fatal("expected 邮件 to allow email")
	}
	if !pushChannelAllowed([]string{"mail"}, "email") {
		t.Fatal("expected mail to allow email")
	}
}

func TestNormalizePushChannels(t *testing.T) {
	got := normalizePushChannels([]string{"邮件", " mail ", "wechat", "微信", ""})
	want := []string{"email", "wechat"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
