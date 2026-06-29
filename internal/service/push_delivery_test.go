package service

import (
	"testing"

	"secmgmt_go/internal/config"
	"secmgmt_go/internal/domain/entity"
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

func TestPushConfigAllowedMatchesSelectedConfigID(t *testing.T) {
	emailConfig := entity.PushConfig{ID: 7, ProviderType: "email"}
	wechatConfig := entity.PushConfig{ID: 8, ProviderType: "wechat"}

	if !pushConfigAllowed([]string{"push-config:7"}, emailConfig) {
		t.Fatal("expected selected push config id to be allowed")
	}
	if pushConfigAllowed([]string{"push-config:7"}, wechatConfig) {
		t.Fatal("expected unselected push config id to be skipped")
	}
	if pushConfigAllowed([]string{"email"}, emailConfig) {
		t.Fatal("expected legacy email channel selector to be skipped")
	}
}

func TestNormalizePushConfigSelectors(t *testing.T) {
	got := normalizePushConfigSelectors([]string{"邮件", "push-config:7", "config:8", "email", "push-config:7", ""})
	want := []string{"push-config:7", "push-config:8"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
