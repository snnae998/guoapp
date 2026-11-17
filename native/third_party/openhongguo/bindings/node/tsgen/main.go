// Command tsgen 根据 sdk 和 sdk/model 的源码生成 TypeScript 类型，
// 供 Node.js（types.d.ts）和 React Native（src/types.ts）两个包使用。
//
// 结构体按 JSON 标签生成接口：带 omitempty 的字段是可选的，标为 "-" 的字段
// 不生成。没有 JSON 标签的查询参数结构体，字段名改为首字母小写的驼峰，
// 和 Go 解码时不区分大小写的规则对应。Go 注释原样带成 JSDoc。
//
// 用法：go run ./bindings/node/tsgen > bindings/node/types.d.ts
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/beilunyang/openhongguo/bindings/internal/schema"
)

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	out, err := generate(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "tsgen:", err)
		os.Exit(1)
	}
	os.Stdout.WriteString(out)
}

// generate 返回仓库根目录 root 对应的 types.d.ts 内容。
func generate(root string) (string, error) {
	s, err := schema.Load(root)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteString("// 由 go run ./bindings/node/tsgen 生成，请勿手工修改。\n")
	b.WriteString("// 返回值的字段名与 Go SDK 的 JSON 一致（如 has_more、cover_url）；\n")
	b.WriteString("// 查询参数用驼峰（如 pageSize）。\n\n")

	b.WriteString("/** Page 是一页匿名结果。 */\n")
	b.WriteString("export interface Page<T> {\n  items: T[];\n  cursor?: string;\n  has_more: boolean;\n}\n\n")

	for _, t := range s.Model {
		writeType(&b, t)
	}
	for _, t := range s.Queries {
		writeType(&b, t)
	}
	return b.String(), nil
}

func writeType(b *strings.Builder, t schema.Type) {
	if !t.Struct {
		var values []string
		for _, c := range t.Consts {
			line := fmt.Sprintf("%s = %s", c.GoName, c.Value)
			if c.Note != "" {
				line += "：" + c.Note
			}
			values = append(values, line)
		}
		writeDoc(b, "", t.Doc, values)
		fmt.Fprintf(b, "export type %s = %s;\n\n", t.Name, tsType(t.Underlying))
		return
	}
	writeDoc(b, "", t.Doc, nil)
	var fields strings.Builder
	for _, f := range t.Fields {
		writeDoc(&fields, "  ", f.Doc, nil)
		opt := ""
		if f.Optional {
			opt = "?"
		}
		fmt.Fprintf(&fields, "  %s%s: %s;\n", f.JSONName, opt, tsType(f.Type))
	}
	ext := ""
	if len(t.Embeds) > 0 {
		ext = " extends " + strings.Join(t.Embeds, ", ")
	}
	fmt.Fprintf(b, "export interface %s%s {\n%s}\n\n", t.Name, ext, fields.String())
}

func writeDoc(b *strings.Builder, indent, doc string, extra []string) {
	doc = strings.TrimSpace(doc)
	if doc == "" && len(extra) == 0 {
		return
	}
	b.WriteString(indent + "/**\n")
	if doc != "" {
		for _, l := range strings.Split(doc, "\n") {
			b.WriteString(strings.TrimRight(indent+" * "+strings.ReplaceAll(l, "*/", "* /"), " ") + "\n")
		}
	}
	if len(extra) > 0 {
		if doc != "" {
			b.WriteString(indent + " *\n")
		}
		b.WriteString(indent + " * 取值：\n")
		for _, l := range extra {
			b.WriteString(indent + " * - " + l + "\n")
		}
	}
	b.WriteString(indent + " */\n")
}

func tsType(r schema.Ref) string {
	switch r.Kind {
	case schema.String, schema.Time: // Time 是 RFC 3339 字符串
		return "string"
	case schema.Bool:
		return "boolean"
	case schema.Int, schema.Int64, schema.Float:
		return "number"
	case schema.List:
		inner := tsType(*r.Elem)
		if strings.Contains(inner, " ") {
			inner = "(" + inner + ")"
		}
		return inner + "[]"
	case schema.Map:
		return "Record<string, " + tsType(*r.Elem) + ">"
	}
	return r.Name
}
