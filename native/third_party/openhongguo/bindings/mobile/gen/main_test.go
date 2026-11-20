package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestGeneratedUpToDate 保证提交的 Types.kt 和 Types.swift 与 Go 源码一致。
// 不一致时在仓库根目录运行 go run ./bindings/mobile/gen。
func TestGeneratedUpToDate(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	out, err := generate(root)
	if err != nil {
		t.Fatal(err)
	}
	for path, want := range out {
		got, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != want {
			t.Errorf("%s is stale: run go run ./bindings/mobile/gen", path)
		}
	}
}

func TestCamel(t *testing.T) {
	for in, want := range map[string]string{
		"series_id": "seriesId", "cover_url": "coverUrl", "pageSize": "pageSize", "key": "key",
	} {
		if got := camel(in); got != want {
			t.Errorf("camel(%q) = %q, want %q", in, got, want)
		}
	}
}
