// Command gen 根据 sdk 和 sdk/model 的源码，生成 Android 绑定的 Types.kt、
// iOS 绑定的 Types.swift 和 Flutter 包的 types.dart。
//
// 属性名由 JSON 名转成驼峰（series_id → seriesId），两个平台一致。Go 里的
// 零值缺省规则照搬过来：除了 Go 的指针字段可以为空，其余字段缺省时取零值，
// 不用可空类型。枚举生成为包着整数的值类型，已知的取值是它的静态常量。
// 查询参数的字段全部可空，为空表示不传。
//
// 用法：go run ./bindings/mobile/gen（在仓库根目录运行）
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/beilunyang/openhongguo/bindings/internal/schema"
)

// 输出文件，相对仓库根目录。
const (
	kotlinFile = "bindings/mobile/android/src/main/kotlin/dev/openhongguo/Types.kt"
	swiftFile  = "bindings/mobile/ios/Sources/Openhongguo/Types.swift"
	dartFile   = "bindings/flutter/lib/src/types.dart"
)

func main() {
	out, err := generate(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, "gen:", err)
		os.Exit(1)
	}
	for path, content := range out {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "gen:", err)
			os.Exit(1)
		}
	}
}

// generate 返回仓库根目录 root 对应的输出，键是相对 root 的路径。
func generate(root string) (map[string]string, error) {
	s, err := schema.Load(root)
	if err != nil {
		return nil, err
	}
	return map[string]string{
		filepath.FromSlash(kotlinFile): kotlin(s),
		filepath.FromSlash(swiftFile):  swift(s),
		filepath.FromSlash(dartFile):   dart(s),
	}, nil
}

const header = "由 go run ./bindings/mobile/gen 生成，请勿手工修改。"

// camel 把 JSON 名转成驼峰：series_id → seriesId，pageSize 保持不变。
func camel(json string) string {
	parts := strings.Split(json, "_")
	var b strings.Builder
	for i, p := range parts {
		if p == "" {
			continue
		}
		r := []rune(p)
		if i == 0 {
			r[0] = unicode.ToLower(r[0])
		} else {
			r[0] = unicode.ToUpper(r[0])
		}
		b.WriteString(string(r))
	}
	return b.String()
}

// constName 是去掉枚举公共前缀后的常量名单词，如 ["Short", "Series"]。
func constWords(t schema.Type, c schema.Const) []string {
	return schema.Words(strings.TrimPrefix(c.GoName, schema.EnumPrefix(t.Consts)))
}

func docLines(doc string) []string {
	doc = strings.TrimSpace(doc)
	if doc == "" {
		return nil
	}
	return strings.Split(doc, "\n")
}

func indentLines(lines []string, indent, prefix string) string {
	var b strings.Builder
	for _, l := range lines {
		b.WriteString(strings.TrimRight(indent+prefix+l, " ") + "\n")
	}
	return b.String()
}

// isStruct 报告 r 是否引用 model 里的结构体。
func isStruct(s *schema.Schema, r schema.Ref) bool {
	if r.Kind != schema.Named {
		return false
	}
	t, ok := s.Lookup(r.Name)
	return ok && t.Struct
}

// enumOf 返回 r 引用的枚举的底层类型。
func enumOf(s *schema.Schema, r schema.Ref) (schema.Ref, bool) {
	if r.Kind != schema.Named {
		return schema.Ref{}, false
	}
	t, ok := s.Lookup(r.Name)
	if !ok || t.Struct {
		return schema.Ref{}, false
	}
	return t.Underlying, true
}
