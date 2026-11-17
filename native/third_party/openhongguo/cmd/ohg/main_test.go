package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateConfigCopiesProfileAndHistoryOnce(t *testing.T) {
	base := t.TempDir()
	oldDir, newDir := filepath.Join(base, "hgdrama"), filepath.Join(base, "openhongguo")
	if err := os.MkdirAll(oldDir, 0700); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"anonymous.json": "profile", "history.json": "history", "other.txt": "x"} {
		if err := os.WriteFile(filepath.Join(oldDir, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := migrateConfig(oldDir, newDir); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"anonymous.json": "profile", "history.json": "history"} {
		got, err := os.ReadFile(filepath.Join(newDir, name))
		if err != nil || string(got) != want {
			t.Fatalf("%s = %q err %v", name, got, err)
		}
		if info, _ := os.Stat(filepath.Join(newDir, name)); info.Mode().Perm() != 0600 {
			t.Fatalf("%s mode %o", name, info.Mode().Perm())
		}
	}
	if _, err := os.Stat(filepath.Join(newDir, "other.txt")); err == nil {
		t.Fatal("unrelated file copied")
	}
	// Once the new directory exists, the old one is left alone.
	if err := os.WriteFile(filepath.Join(oldDir, "history.json"), []byte("newer"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := migrateConfig(oldDir, newDir); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(newDir, "history.json")); string(got) != "history" {
		t.Fatalf("history overwritten: %q", got)
	}
}

func TestMigrateConfigWithoutOldDir(t *testing.T) {
	base := t.TempDir()
	if err := migrateConfig(filepath.Join(base, "hgdrama"), filepath.Join(base, "openhongguo")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(base, "openhongguo")); err == nil {
		t.Fatal("new dir created without anything to migrate")
	}
}
