package main

import (
	"fmt"
	"strings"

	"github.com/beilunyang/openhongguo/bindings/internal/schema"
)

// dartRenames 避开和 Dart、Flutter 自带类型的重名：dart:core 的 Stream。
var dartRenames = map[string]string{"Stream": "VideoStream"}

var dartKeywords = map[string]bool{
	"assert": true, "break": true, "case": true, "catch": true, "class": true, "const": true,
	"continue": true, "default": true, "do": true, "else": true, "enum": true, "extends": true,
	"false": true, "final": true, "finally": true, "for": true, "if": true, "in": true, "is": true,
	"new": true, "null": true, "rethrow": true, "return": true, "super": true, "switch": true,
	"this": true, "throw": true, "true": true, "try": true, "var": true, "void": true, "while": true,
	"with": true,
}

func dartTypeName(name string) string {
	if n, ok := dartRenames[name]; ok {
		return n
	}
	return name
}

// dartName 是属性或常量名；和关键字重名时加上 Value，如 newValue。
func dartName(name string) string {
	if dartKeywords[name] {
		return name + "Value"
	}
	return name
}

func dart(s *schema.Schema) string {
	var b strings.Builder
	b.WriteString("// " + header + "\n\n")
	b.WriteString("part of '../openhongguo.dart';\n\n")
	for _, t := range s.Model {
		if t.Struct {
			dartStruct(&b, s, t)
		} else {
			dartEnum(&b, t)
		}
	}
	for _, t := range s.Queries {
		dartQuery(&b, s, t)
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

func dartDoc(b *strings.Builder, indent, doc string) {
	b.WriteString(indentLines(docLines(doc), indent, "/// "))
}

func dartType(s *schema.Schema, r schema.Ref) string {
	var t string
	switch r.Kind {
	case schema.String, schema.Time:
		t = "String"
	case schema.Bool:
		t = "bool"
	case schema.Int, schema.Int64:
		t = "int"
	case schema.Float:
		t = "double"
	case schema.List:
		t = "List<" + dartType(s, *r.Elem) + ">"
	case schema.Map:
		t = "Map<String, " + dartType(s, *r.Elem) + ">"
	default:
		t = dartTypeName(r.Name)
	}
	if r.Pointer {
		t += "?"
	}
	return t
}

// dartZero 是字段的默认值，必须是常量表达式。
func dartZero(s *schema.Schema, r schema.Ref) string {
	if r.Pointer {
		return "null"
	}
	switch r.Kind {
	case schema.String, schema.Time:
		return "''"
	case schema.Bool:
		return "false"
	case schema.Int, schema.Int64:
		return "0"
	case schema.Float:
		return "0.0"
	case schema.List:
		return "const []"
	case schema.Map:
		return "const {}"
	}
	if u, ok := enumOf(s, r); ok {
		return "const " + dartTypeName(r.Name) + "(" + dartZero(s, u) + ")"
	}
	return "const " + dartTypeName(r.Name) + "()"
}

// dartDecode 返回把 JSON 值 v（dynamic）解码成 r 的表达式。辅助函数见 json.dart。
func dartDecode(s *schema.Schema, r schema.Ref, v string) string {
	switch r.Kind {
	case schema.String, schema.Time:
		return "_asString(" + v + ")"
	case schema.Bool:
		return "_asBool(" + v + ")"
	case schema.Int, schema.Int64:
		return "_asInt(" + v + ")"
	case schema.Float:
		return "_asDouble(" + v + ")"
	case schema.List:
		return "_asList(" + v + ", (e) => " + dartDecode(s, *r.Elem, "e") + ")"
	case schema.Map:
		return "_asMapOf(" + v + ", (e) => " + dartDecode(s, *r.Elem, "e") + ")"
	}
	name := dartTypeName(r.Name)
	if u, ok := enumOf(s, r); ok {
		return name + "(" + dartDecode(s, u, v) + ")"
	}
	if r.Pointer {
		return "(" + v + " is Map ? " + name + ".fromJson(_asMap(" + v + ")) : null)"
	}
	return name + ".fromJson(_asMap(" + v + "))"
}

// dartEncode 返回把 Dart 值 x（不为 null）编码成 JSON 值的表达式。
func dartEncode(s *schema.Schema, r schema.Ref, x string) string {
	switch r.Kind {
	case schema.String, schema.Time, schema.Bool, schema.Int, schema.Int64, schema.Float:
		return x
	case schema.List:
		inner := dartEncode(s, *r.Elem, "e")
		if inner == "e" {
			return x
		}
		return x + ".map((e) => " + inner + ").toList()"
	case schema.Map:
		inner := dartEncode(s, *r.Elem, "e")
		if inner == "e" {
			return x
		}
		return x + ".map((k, e) => MapEntry(k, " + inner + "))"
	}
	if _, ok := enumOf(s, r); ok {
		return x + ".value"
	}
	return x + ".toJson()"
}

func dartStruct(b *strings.Builder, s *schema.Schema, t schema.Type) {
	fields := s.AllFields(t)
	name := dartTypeName(t.Name)
	dartDoc(b, "", t.Doc)
	fmt.Fprintf(b, "class %s {\n", name)
	for _, f := range fields {
		dartDoc(b, "  ", f.Doc)
		fmt.Fprintf(b, "  final %s %s;\n", dartType(s, f.Type), dartName(camel(f.JSONName)))
	}
	b.WriteString("\n")
	fmt.Fprintf(b, "  const %s({\n", name)
	for _, f := range fields {
		if f.Type.Pointer {
			fmt.Fprintf(b, "    this.%s,\n", dartName(camel(f.JSONName)))
		} else {
			fmt.Fprintf(b, "    this.%s = %s,\n", dartName(camel(f.JSONName)), dartZero(s, f.Type))
		}
	}
	b.WriteString("  });\n\n")

	b.WriteString("  /// 从 Go SDK 输出的 JSON 解码；缺省的字段取零值。\n")
	fmt.Fprintf(b, "  factory %s.fromJson(Map<String, dynamic> j) => %s(\n", name, name)
	for _, f := range fields {
		fmt.Fprintf(b, "        %s: %s,\n", dartName(camel(f.JSONName)), dartDecode(s, f.Type, fmt.Sprintf("j['%s']", f.JSONName)))
	}
	b.WriteString("      );\n\n")

	b.WriteString("  /// 编码成与 Go SDK 相同的 JSON。\n")
	b.WriteString("  Map<String, dynamic> toJson() => {\n")
	for _, f := range fields {
		prop := dartName(camel(f.JSONName))
		if f.Type.Pointer {
			fmt.Fprintf(b, "        if (%s != null) '%s': %s,\n", prop, f.JSONName, dartEncode(s, f.Type, prop+"!"))
		} else {
			fmt.Fprintf(b, "        '%s': %s,\n", f.JSONName, dartEncode(s, f.Type, prop))
		}
	}
	b.WriteString("      };\n}\n\n")
}

func dartEnum(b *strings.Builder, t schema.Type) {
	var raw string
	switch t.Underlying.Kind {
	case schema.String:
		raw = "String"
	case schema.Int, schema.Int64:
		raw = "int"
	default:
		panic("gen: unsupported enum " + t.Name)
	}
	name := dartTypeName(t.Name)
	dartDoc(b, "", t.Doc)
	fmt.Fprintf(b, "class %s {\n", name)
	fmt.Fprintf(b, "  final %s value;\n\n", raw)
	fmt.Fprintf(b, "  const %s(this.value);\n", name)
	if len(t.Consts) > 0 {
		b.WriteString("\n")
	}
	for _, c := range t.Consts {
		dartDoc(b, "  ", c.Note)
		words := constWords(t, c)
		words[0] = strings.ToLower(words[0])
		fmt.Fprintf(b, "  static const %s = %s(%s);\n", dartName(strings.Join(words, "")), name, c.Value)
	}
	b.WriteString("\n")
	fmt.Fprintf(b, "  @override\n  bool operator ==(Object other) => other is %s && other.value == value;\n\n", name)
	b.WriteString("  @override\n  int get hashCode => value.hashCode;\n\n")
	fmt.Fprintf(b, "  @override\n  String toString() => '%s($value)';\n}\n\n", name)
}

func dartQuery(b *strings.Builder, s *schema.Schema, t schema.Type) {
	name := dartTypeName(t.Name)
	dartDoc(b, "", t.Doc)
	fmt.Fprintf(b, "class %s {\n", name)
	for _, f := range t.Fields {
		ft := f.Type
		ft.Pointer = false
		dartDoc(b, "  ", f.Doc)
		fmt.Fprintf(b, "  final %s? %s;\n", dartType(s, ft), dartName(camel(f.JSONName)))
	}
	b.WriteString("\n")
	fmt.Fprintf(b, "  const %s({", name)
	for i, f := range t.Fields {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(b, "this.%s", dartName(camel(f.JSONName)))
	}
	b.WriteString("});\n\n")
	fmt.Fprintf(b, "  %s copyWith({", name)
	for i, f := range t.Fields {
		ft := f.Type
		ft.Pointer = false
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(b, "%s? %s", dartType(s, ft), dartName(camel(f.JSONName)))
	}
	fmt.Fprintf(b, "}) =>\n      %s(", name)
	for i, f := range t.Fields {
		if i > 0 {
			b.WriteString(", ")
		}
		p := dartName(camel(f.JSONName))
		fmt.Fprintf(b, "%s: %s ?? this.%s", p, p, p)
	}
	b.WriteString(");\n\n")
	b.WriteString("  Map<String, dynamic> toJson() => {\n")
	for _, f := range t.Fields {
		ft := f.Type
		ft.Pointer = false
		prop := dartName(camel(f.JSONName))
		fmt.Fprintf(b, "        if (%s != null) '%s': %s,\n", prop, f.JSONName, dartEncode(s, ft, prop+"!"))
	}
	b.WriteString("      };\n}\n\n")
}
