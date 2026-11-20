package history

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRecordGetListAndClear(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "history.json")
	s := NewStore(path)
	if _, ok, err := s.Get(ctx, "1"); err != nil || ok {
		t.Fatalf("empty get ok=%v err=%v", ok, err)
	}
	older := Entry{SeriesID: "1", Title: "A", Episode: 3, Episodes: 10, UpdatedAt: time.Unix(100, 0).UTC()}
	newer := Entry{SeriesID: "2", Title: "B", Episode: 1, Finished: true, UpdatedAt: time.Unix(200, 0).UTC()}
	for _, e := range []Entry{older, newer} {
		if err := s.Record(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("mode = %v err = %v", info.Mode().Perm(), err)
	}
	got, ok, err := NewStore(path).Get(ctx, "1")
	if err != nil || !ok || got != older {
		t.Fatalf("get = %+v ok=%v err=%v", got, ok, err)
	}
	list, err := s.List(ctx)
	if err != nil || len(list) != 2 || list[0].SeriesID != "2" {
		t.Fatalf("list = %+v err=%v", list, err)
	}
	older.Episode, older.UpdatedAt = 4, time.Time{}
	if err := s.Record(ctx, older); err != nil {
		t.Fatal(err)
	}
	if got, _, _ := s.Get(ctx, "1"); got.Episode != 4 || got.UpdatedAt.IsZero() {
		t.Fatalf("updated = %+v", got)
	}
	if err := s.Clear(ctx); err != nil {
		t.Fatal(err)
	}
	if list, _ := s.List(ctx); len(list) != 0 {
		t.Fatalf("after clear = %+v", list)
	}
}

func TestRecordRejectsBadEntryAndSymlink(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s := NewStore(filepath.Join(dir, "history.json"))
	if err := s.Record(ctx, Entry{SeriesID: "1"}); err == nil {
		t.Fatal("entry without episode accepted")
	}
	link := filepath.Join(dir, "link.json")
	if err := os.Symlink(filepath.Join(dir, "elsewhere"), link); err != nil {
		t.Fatal(err)
	}
	if err := NewStore(link).Record(ctx, Entry{SeriesID: "1", Episode: 1}); err == nil {
		t.Fatal("symlink history accepted")
	}
}
