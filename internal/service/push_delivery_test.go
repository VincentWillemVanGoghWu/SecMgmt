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
