package filestore

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/beilunyang/openhongguo/sdk"
)

func TestProfileFileModeIsOwnerReadWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "anonymous.json")
	store := New(path)
	profile, err := sdk.NewAnonymousDeviceProfile(nil, func() time.Time { return time.Unix(1, 0).UTC() })
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(context.Background(), profile); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("mode = %o", info.Mode().Perm())
	}
	loaded, err := store.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if loaded.CDID != profile.CDID || loaded.DeviceToken != "" {
		t.Fatalf("loaded = %+v", loaded)
	}
}
