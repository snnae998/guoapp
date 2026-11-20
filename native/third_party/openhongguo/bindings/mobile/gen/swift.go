package main

import (
	"fmt"
	"strings"

	"github.com/beilunyang/openhongguo/bindings/internal/schema"
)

var swiftKeywords = map[string]bool{
	"as": true, "associatedtype": true, "break": true, "case": true, "catch": true, "class": true,
	"continue": true, "default": true, "defer": true, "deinit": true, "do": true, "else": true,
	"enum": true, "extension": true, "fallthrough": true, "false": true, "fileprivate": true,
	"for": true, "func": true, "guard": true, "if": true, "import": true, "in": true, "init": true,
	"inout": true, "internal": true, "is": true, "let": true, "nil": true, "operator": true,
	"private": true, "protocol": true, "public": true, "repeat": true, "rethrows": true,
	"return": true, "self": true, "static": true, "struct": true, "subscript": true, "super": true,
	"switch": true, "throw": true, "throws": true, "true": true, "try": true, "var": true,
	"where": true, "while": true,
}

func swiftName(name string) string {
	if swiftKeywords[name] {
		return "`" + name + "`"
	}
	return name
}

func swift(s *schema.Schema) string {
	var b strings.Builder
	b.WriteString("// " + header + "\n\n")
	b.WriteString("import Foundation\n\n")
	for _, t := range s.Model {
		if t.Struct {
			swiftStruct(&b, s, t.Name, t.Doc, s.AllFields(t), false)
		} else {
			swiftEnum(&b, t)
		}
	}
	for _, t := range s.Queries {
		swiftStruct(&b, s, t.Name, t.Doc, t.Fields, true)
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

func swiftDoc(b *strings.Builder, indent, doc string) {
	b.WriteString(indentLines(docLines(doc), indent, "/// "))
}

func swiftType(s *schema.Schema, r schema.Ref) string {
	var t string
	switch r.Kind {
	case schema.String, schema.Time:
		t = "String"
	case schema.Bool:
		t = "Bool"
	case schema.Int, schema.Int64:
		t = "Int"
	case schema.Float:
		t = "Double"
	case schema.List:
		t = "[" + swiftType(s, *r.Elem) + "]"
	case schema.Map:
		t = "[String: " + swiftType(s, *r.Elem) + "]"
	default:
		t = r.Name
	}
	if r.Pointer {
		t += "?"
	}
	return t
}

func swiftZero(s *schema.Schema, r schema.Ref) string {
	if r.Pointer {
		return "nil"
	}
	switch r.Kind {
	case schema.String, schema.Time:
		return `""`
	case schema.Bool:
		return "false"
	case schema.Int, schema.Int64:
		return "0"
	case schema.Float:
		return "0"
	case schema.List:
		return "[]"
	case schema.Map:
		return "[:]"
	}
	if u, ok := enumOf(s, r); ok {
		return r.Name + "(rawValue: " + swiftZero(s, u) + ")"
	}
	return r.Name + "()"
}

// swiftStruct 生成一个结构体。query 为真时生成查询参数：只编码，字段可空。
func swiftStruct(b *strings.Builder, s *schema.Schema, name, doc string, fields []schema.Field, query bool) {
	type prop struct {
		name, json, typ, zero, doc string
	}
	var props []prop
	needKeys := false
	for _, f := range fields {
		p := prop{name: camel(f.JSONName), json: f.JSONName, doc: f.Doc}
		ft := f.Type
		if query {
			ft.Pointer = false
			p.typ, p.zero = swiftType(s, ft)+"?", "nil"
		} else {
			p.typ, p.zero = swiftType(s, ft), swiftZero(s, ft)
		}
		if p.name != p.json {
			needKeys = true
		}
		props = append(props, p)
	}

	swiftDoc(b, "", doc)
	conforms := "Codable, Hashable, Sendable"
	if query {
		conforms = "Encodable, Hashable, Sendable"
	}
	fmt.Fprintf(b, "public struct %s: %s {\n", name, conforms)
	for _, p := range props {
		swiftDoc(b, "    ", p.doc)
		fmt.Fprintf(b, "    public var %s: %s\n", swiftName(p.name), p.typ)
	}

	b.WriteString("\n    public init(")
	for i, p := range props {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(b, "%s: %s = %s", p.name, p.typ, p.zero)
	}
	b.WriteString(") {\n")
	for _, p := range props {
		fmt.Fprintf(b, "        self.%s = %s\n", p.name, swiftName(p.name))
	}
	b.WriteString("    }\n")

	if needKeys || !query {
		b.WriteString("\n    enum CodingKeys: String, CodingKey {\n")
		for _, p := range props {
			if p.name == p.json {
				fmt.Fprintf(b, "        case %s\n", swiftName(p.name))
			} else {
				fmt.Fprintf(b, "        case %s = %q\n", swiftName(p.name), p.json)
			}
		}
		b.WriteString("    }\n")
	}

	if !query {
		// 缺省或为 null 的字段取零值，与 Go 的 encoding/json 一致。
		b.WriteString("\n    public init(from decoder: Decoder) throws {\n")
		b.WriteString("        let c = try decoder.container(keyedBy: CodingKeys.self)\n")
		for _, p := range props {
			base := strings.TrimSuffix(p.typ, "?")
			line := fmt.Sprintf("        self.%s = try c.decodeIfPresent(%s.self, forKey: .%s)", p.name, base, p.name)
			if p.zero != "nil" {
				line += " ?? " + p.zero
			}
			b.WriteString(line + "\n")
		}
		b.WriteString("    }\n")
	}
	b.WriteString("}\n\n")
}

func swiftEnum(b *strings.Builder, t schema.Type) {
	swiftDoc(b, "", t.Doc)
	var raw string
	switch t.Underlying.Kind {
	case schema.String:
		raw = "String"
	case schema.Int, schema.Int64:
		raw = "Int"
	default:
		panic("gen: unsupported enum " + t.Name)
	}
	fmt.Fprintf(b, "public struct %s: RawRepresentable, Codable, Hashable, Sendable {\n", t.Name)
	fmt.Fprintf(b, "    public var rawValue: %s\n\n", raw)
	fmt.Fprintf(b, "    public init(rawValue: %s) {\n        self.rawValue = rawValue\n    }\n", raw)
	if len(t.Consts) > 0 {
		b.WriteString("\n")
	}
	for _, c := range t.Consts {
		swiftDoc(b, "    ", c.Note)
		words := constWords(t, c)
		words[0] = strings.ToLower(words[0])
		fmt.Fprintf(b, "    public static let %s = %s(rawValue: %s)\n", swiftName(strings.Join(words, "")), t.Name, c.Value)
	}
	b.WriteString("}\n\n")
}
