package sdk

import (
	"testing"
	"time"
)

func TestProfileGenerationIsRandomAndSecretFree(t *testing.T) {
	a, err := NewAnonymousDeviceProfile(nil, func() time.Time { return time.Unix(10, 0).UTC() })
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewAnonymousDeviceProfile(nil, func() time.Time { return time.Unix(10, 0).UTC() })
	if err != nil {
		t.Fatal(err)
	}
	if a.CDID == b.CDID || a.OpenUDID == b.OpenUDID {
		t.Fatal("profiles were not random")
	}
	if a.DeviceID != "" || a.InstallID != "" || a.DeviceToken != "" {
		t.Fatal("local profile invented a server identity")
	}
	if len(a.CDID) != 36 || a.CreatedAt.IsZero() {
		t.Fatalf("profile = %+v", a)
	}
}
