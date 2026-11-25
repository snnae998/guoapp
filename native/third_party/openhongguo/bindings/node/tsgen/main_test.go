package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestTypesUpToDate 保证 Node.js 和 React Native 包里提交的类型与 Go 源码一致。
// 不一致时运行 go run ./bindings/node/tsgen > <文件>。
func TestTypesUpToDate(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	want, err := generate(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"bindings/node/types.d.ts", "bindings/react-native/src/types.ts"} {
		got, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != want {
			t.Errorf("%s is stale: run go run ./bindings/node/tsgen > %s", path, path)
		}
	}
}
