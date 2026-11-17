package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/beilunyang/openhongguo/bindings/internal/dispatch"
)

// TestGeneratedUpToDate 保证提交的文档与 Go 源码一致。
// 不一致时在仓库根目录运行 go run ./bindings/docgen。
func TestGeneratedUpToDate(t *testing.T) {
	root := filepath.Join("..", "..")
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
			t.Errorf("%s is stale: run go run ./bindings/docgen", path)
		}
	}
}

// TestGroupsCoverAPI 保证方法表列出了 Client 的每个公开方法和每个绑定方法。
func TestGroupsCoverAPI(t *testing.T) {
	a, err := loadAPI(filepath.Join("..", "..", "sdk"))
	if err != nil {
		t.Fatal(err)
	}
	listed := map[string]bool{}
	bound := map[string]bool{}
	for _, g := range groups {
		for _, m := range g.methods {
			if listed[m] {
				t.Errorf("%s is listed twice", m)
			}
			listed[m] = true
			bound[bindingName(m)] = true
		}
	}
	for _, typ := range a.pkg.Types {
		if typ.Name != "Client" {
			continue
		}
		for _, m := range typ.Methods {
			if !listed[m.Name] {
				t.Errorf("Client.%s is missing from groups", m.Name)
			}
		}
	}
	for _, m := range dispatch.Methods() {
		if !bound[m] {
			t.Errorf("binding method %s is missing from groups", m)
		}
	}
}

// TestErrorCodesCoverSentinels 保证每个哨兵错误都有错误码。
func TestErrorCodesCoverSentinels(t *testing.T) {
	a, err := loadAPI(filepath.Join("..", "..", "sdk"))
	if err != nil {
		t.Fatal(err)
	}
	codes := map[string]bool{}
	for _, e := range dispatch.ErrorCodes {
		codes[e.Code] = true
	}
	for _, name := range a.order {
		if !codes[codeOf(name)] {
			t.Errorf("%s has no error code in dispatch.ErrorCodes", name)
		}
		if a.vars[name] == "" {
			t.Errorf("%s has no doc comment", name)
		}
	}
}

func TestParamName(t *testing.T) {
	for in, want := range map[string]string{
		"seriesID": "seriesId", "seriesIDs": "seriesIds", "opts": "options", "q": "query", "offset": "offset",
	} {
		if got := paramName(in); got != want {
			t.Errorf("paramName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCJKJoin(t *testing.T) {
	if got := cjkJoin("热搜榜、 漫剧 ok 中文 A； DefaultTab"); got != "热搜榜、漫剧 ok 中文 A；DefaultTab" {
		t.Errorf("cjkJoin = %q", got)
	}
}
