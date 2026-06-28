package util

import "testing"

func TestDeviceSecretRoundTrip(t *testing.T) {
	encrypted, err := EncryptDeviceSecret("test-secret", "P@ssw0rd!")
	if err != nil {
		t.Fatalf("EncryptDeviceSecret() error = %v", err)
	}
	if encrypted == "" || encrypted == "P@ssw0rd!" {
		t.Fatalf("EncryptDeviceSecret() returned unexpected value %q", encrypted)
	}

	resolved, err := ResolveDeviceSecret("test-secret", encrypted)
	if err != nil {
		t.Fatalf("ResolveDeviceSecret() error = %v", err)
	}
	if resolved != "P@ssw0rd!" {
		t.Fatalf("ResolveDeviceSecret() = %q, want %q", resolved, "P@ssw0rd!")
	}
}

func TestResolveDeviceSecretSupportsLegacyPlaintext(t *testing.T) {
	resolved, err := ResolveDeviceSecret("test-secret", "legacy-plain-password")
	if err != nil {
		t.Fatalf("ResolveDeviceSecret() error = %v", err)
	}
	if resolved != "legacy-plain-password" {
		t.Fatalf("ResolveDeviceSecret() = %q, want %q", resolved, "legacy-plain-password")
	}
}
