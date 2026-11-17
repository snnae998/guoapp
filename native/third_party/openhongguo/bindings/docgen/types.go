package main

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/beilunyang/openhongguo/bindings/internal/schema"
)

// typesDoc 生成类型参考 docs/types.md。
func typesDoc(s *schema.Schema) string {
	var b strings.Builder
	b.WriteString("# 类型参考\n\n" + note + "\n\n")
	b.WriteString(`各语言 SDK 的返回值和查询参数都用下面这些类型，类型名相同。只有 Flutter 例外：分页结果 ` + "`Page`" + ` 叫 ` + "`PagedResult`" + `，流 ` + "`Stream`" + ` 叫 ` + "`VideoStream`" + `。

字段名有两种写法。Go 的 JSON、Node.js 和 React Native 用下划线风格（` + "`cover_url`" + `）；Kotlin、Swift 和 Dart 用驼峰（` + "`coverUrl`" + `）。

字段缺省时取零值（空字符串、0、空列表）。类型标有“或 null”的字段例外，它们在 Go 里是指针，可以为空。

## 返回值
`)
	for _, t := range s.Model {
		fmt.Fprintf(&b, "\n### %s\n\n", t.Name)
		if d := paragraphs(t.Doc); d != "" {
			b.WriteString(d + "\n\n")
		}
		if !t.Struct {
			enumTable(&b, t)
			continue
		}
		for _, e := range t.Embeds {
			fmt.Fprintf(&b, "包含 [%s](#%s) 的全部字段。\n\n", e, strings.ToLower(e))
		}
		if len(t.Fields) == 0 {
			continue
		}
		b.WriteString("| 字段 | 驼峰 | 类型 | 说明 |\n|---|---|---|---|\n")
		for _, f := range t.Fields {
			fmt.Fprintf(&b, "| `%s` | `%s` | %s | %s |\n", f.JSONName, camel(f.JSONName), typeRef(f.Type), cell(f.Doc))
		}
	}
	b.WriteString("\n## 查询参数\n\n查询参数的字段全部可选，不填表示不传。字段名在各语言里都是驼峰。\n")
	for _, t := range s.Queries {
		fmt.Fprintf(&b, "\n### %s\n\n", t.Name)
		if d := paragraphs(t.Doc); d != "" {
			b.WriteString(d + "\n\n")
		}
		b.WriteString("| 字段 | 类型 | 说明 |\n|---|---|---|\n")
		for _, f := range t.Fields {
			fmt.Fprintf(&b, "| `%s` | %s | %s |\n", f.JSONName, typeRef(f.Type), cell(f.Doc))
		}
	}
	return b.String()
}

func enumTable(b *strings.Builder, t schema.Type) {
	fmt.Fprintf(b, "取值是 %s，已知的取值如下。服务端新增的取值也会原样保留。\n\n", typeRef(t.Underlying))
	b.WriteString("| 常量 | 值 | 说明 |\n|---|---|---|\n")
	for _, c := range t.Consts {
		fmt.Fprintf(b, "| `%s` | `%s` | %s |\n", c.GoName, c.Value, cell(c.Note))
	}
}

// typeRef 是字段类型的写法，引用 model 的类型时链接到它的小节。
func typeRef(r schema.Ref) string {
	var s string
	switch r.Kind {
	case schema.String:
		s = "string"
	case schema.Bool:
		s = "bool"
	case schema.Int:
		s = "int"
	case schema.Int64:
		s = "int64"
	case schema.Float:
		s = "float"
	case schema.Time:
		s = "时间（RFC 3339 字符串）"
	case schema.Named:
		s = fmt.Sprintf("[%s](#%s)", r.Name, strings.ToLower(r.Name))
	case schema.List:
		s = typeRef(*r.Elem) + " 列表"
	case schema.Map:
		s = "string 到 " + typeRef(*r.Elem) + " 的映射"
	}
	if r.Pointer {
		s += " 或 null"
	}
	return s
}

// paragraphs 把 Go 注释整理成 Markdown 段落。
func paragraphs(doc string) string {
	var out []string
	for _, p := range strings.Split(strings.TrimSpace(doc), "\n\n") {
		if p = cjkJoin(strings.Join(strings.Fields(p), " ")); p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, "\n\n")
}

// camel 把 JSON 名转成驼峰：series_id → seriesId，与 bindings/mobile/gen 一致。
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
