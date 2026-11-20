package main

import (
	"fmt"
	"strings"

	"github.com/beilunyang/openhongguo/bindings/internal/schema"
)

var kotlinKeywords = map[string]bool{
	"as": true, "break": true, "class": true, "continue": true, "do": true, "else": true,
	"false": true, "for": true, "fun": true, "if": true, "in": true, "interface": true,
	"is": true, "null": true, "object": true, "package": true, "return": true, "super": true,
	"this": true, "throw": true, "true": true, "try": true, "typealias": true, "typeof": true,
	"val": true, "var": true, "when": true, "while": true,
}

func kotlinName(json string) string {
	n := camel(json)
	if kotlinKeywords[n] {
		return "`" + n + "`"
	}
	return n
}

func kotlin(s *schema.Schema) string {
	var b strings.Builder
	b.WriteString("// " + header + "\n\n")
	b.WriteString("package dev.openhongguo\n\n")
	b.WriteString("import org.json.JSONArray\n")
	b.WriteString("import org.json.JSONObject\n\n")
	for _, t := range s.Model {
		if t.Struct {
			kotlinStruct(&b, s, t)
		} else {
			kotlinEnum(&b, t)
		}
	}
	for _, t := range s.Queries {
		kotlinQuery(&b, s, t)
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

func kdoc(b *strings.Builder, indent, doc string, extra ...string) {
	lines := docLines(strings.ReplaceAll(doc, "*/", "* /"))
	if len(lines) == 0 && len(extra) == 0 {
		return
	}
	if len(lines) == 1 && len(extra) == 0 {
		b.WriteString(indent + "/** " + lines[0] + " */\n")
		return
	}
	b.WriteString(indent + "/**\n")
	b.WriteString(indentLines(lines, indent, " * "))
	if len(extra) > 0 && len(lines) > 0 {
		b.WriteString(indent + " *\n")
	}
	b.WriteString(indentLines(extra, indent, " * "))
	b.WriteString(indent + " */\n")
}

func kotlinType(s *schema.Schema, r schema.Ref) string {
	var t string
	switch r.Kind {
	case schema.String, schema.Time:
		t = "String"
	case schema.Bool:
		t = "Boolean"
	case schema.Int:
		t = "Int"
	case schema.Int64:
		t = "Long"
	case schema.Float:
		t = "Double"
	case schema.List:
		t = "List<" + kotlinType(s, *r.Elem) + ">"
	case schema.Map:
		t = "Map<String, " + kotlinType(s, *r.Elem) + ">"
	default:
		t = r.Name
	}
	if r.Pointer {
		t += "?"
	}
	return t
}

func kotlinZero(s *schema.Schema, r schema.Ref) string {
	if r.Pointer {
		return "null"
	}
	switch r.Kind {
	case schema.String, schema.Time:
		return `""`
	case schema.Bool:
		return "false"
	case schema.Int:
		return "0"
	case schema.Int64:
		return "0L"
	case schema.Float:
		return "0.0"
	case schema.List:
		return "emptyList()"
	case schema.Map:
		return "emptyMap()"
	}
	if u, ok := enumOf(s, r); ok {
		return r.Name + "(" + kotlinZero(s, u) + ")"
	}
	return r.Name + "()"
}

// kotlinDecode 返回把 JSON 值 v（Any?）解码成 r 的表达式。辅助函数见 Json.kt。
func kotlinDecode(s *schema.Schema, r schema.Ref, v string) string {
	switch r.Kind {
	case schema.String, schema.Time:
		return v + ".asString()"
	case schema.Bool:
		return v + ".asBoolean()"
	case schema.Int:
		return v + ".asInt()"
	case schema.Int64:
		return v + ".asLong()"
	case schema.Float:
		return v + ".asDouble()"
	case schema.List:
		return v + ".asList { " + kotlinDecode(s, *r.Elem, "it") + " }"
	case schema.Map:
		return v + ".asMap { " + kotlinDecode(s, *r.Elem, "it") + " }"
	}
	if u, ok := enumOf(s, r); ok {
		return r.Name + "(" + kotlinDecode(s, u, v) + ")"
	}
	if r.Pointer {
		return "(" + v + " as? JSONObject)?.let(" + r.Name + "::fromJson)"
	}
	return r.Name + ".fromJson(" + v + ".asObject())"
}

// kotlinEncode 返回把 Kotlin 值 x 编码成 JSON 值的表达式。
func kotlinEncode(s *schema.Schema, r schema.Ref, x string) string {
	switch r.Kind {
	case schema.String, schema.Time, schema.Bool, schema.Int, schema.Int64, schema.Float:
		return x
	case schema.List:
		inner := kotlinEncode(s, *r.Elem, "it")
		if inner == "it" {
			return "JSONArray(" + x + ")"
		}
		return "JSONArray(" + x + ".map { " + inner + " })"
	case schema.Map:
		inner := kotlinEncode(s, *r.Elem, "it.value")
		if inner == "it.value" {
			return "JSONObject(" + x + ")"
		}
		return "JSONObject(" + x + ".mapValues { " + inner + " })"
	}
	if _, ok := enumOf(s, r); ok {
		return x + ".value"
	}
	if r.Pointer {
		return x + "?.toJson()"
	}
	return x + ".toJson()"
}

func kotlinStruct(b *strings.Builder, s *schema.Schema, t schema.Type) {
	fields := s.AllFields(t)
	if len(fields) == 0 {
		panic("gen: empty struct " + t.Name)
	}
	kdoc(b, "", t.Doc)
	fmt.Fprintf(b, "data class %s(\n", t.Name)
	for _, f := range fields {
		kdoc(b, "    ", f.Doc)
		fmt.Fprintf(b, "    val %s: %s = %s,\n", kotlinName(f.JSONName), kotlinType(s, f.Type), kotlinZero(s, f.Type))
	}
	b.WriteString(") {\n")
	b.WriteString("    /** 编码成与 Go SDK 相同的 JSON。 */\n")
	b.WriteString("    fun toJson(): JSONObject = JSONObject().apply {\n")
	for _, f := range fields {
		fmt.Fprintf(b, "        put(%q, %s)\n", f.JSONName, kotlinEncode(s, f.Type, kotlinName(f.JSONName)))
	}
	b.WriteString("    }\n\n")
	b.WriteString("    companion object {\n")
	b.WriteString("        /** 从 Go SDK 输出的 JSON 解码；缺省的字段取零值。 */\n")
	fmt.Fprintf(b, "        fun fromJson(o: JSONObject): %s = %s(\n", t.Name, t.Name)
	for _, f := range fields {
		fmt.Fprintf(b, "            %s = %s,\n", kotlinName(f.JSONName), kotlinDecode(s, f.Type, fmt.Sprintf("o.opt(%q)", f.JSONName)))
	}
	b.WriteString("        )\n    }\n}\n\n")
}

func kotlinEnum(b *strings.Builder, t schema.Type) {
	kdoc(b, "", t.Doc)
	var raw, suffix string
	switch t.Underlying.Kind {
	case schema.String:
		raw = "String"
	case schema.Int:
		raw = "Int"
	case schema.Int64:
		raw, suffix = "Long", "L"
	default:
		panic("gen: unsupported enum " + t.Name)
	}
	b.WriteString("@JvmInline\n")
	fmt.Fprintf(b, "value class %s(val value: %s) {\n", t.Name, raw)
	if len(t.Consts) > 0 {
		b.WriteString("    companion object {\n")
		for _, c := range t.Consts {
			kdoc(b, "        ", c.Note)
			words := constWords(t, c)
			for i, w := range words {
				words[i] = strings.ToUpper(w)
			}
			fmt.Fprintf(b, "        val %s = %s(%s%s)\n", strings.Join(words, "_"), t.Name, c.Value, suffix)
		}
		b.WriteString("    }\n")
	}
	b.WriteString("}\n\n")
}

func kotlinQuery(b *strings.Builder, s *schema.Schema, t schema.Type) {
	kdoc(b, "", t.Doc)
	fmt.Fprintf(b, "data class %s(\n", t.Name)
	for _, f := range t.Fields {
		kdoc(b, "    ", f.Doc)
		ft := f.Type
		ft.Pointer = false
		fmt.Fprintf(b, "    val %s: %s? = null,\n", kotlinName(f.JSONName), kotlinType(s, ft))
	}
	b.WriteString(") {\n")
	b.WriteString("    internal fun toJson(): JSONObject = JSONObject().apply {\n")
	for _, f := range t.Fields {
		ft := f.Type
		ft.Pointer = false
		fmt.Fprintf(b, "        %s?.let { put(%q, %s) }\n", kotlinName(f.JSONName), f.JSONName, kotlinEncode(s, ft, "it"))
	}
	b.WriteString("    }\n}\n\n")
}
