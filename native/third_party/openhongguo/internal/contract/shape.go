// Package contract checks that the live service still answers in the shape
// the parsers were written against. The fixtures in testdata/protocol are
// the contract: every field path they contain, with its JSON type, must still
// appear in the live response.
//
// Only shapes are compared and reported, never values, so a report carries
// no URLs, identifiers, or key material.
package contract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Field is one leaf of a JSON document: a path such as
// "data.video_model{}.video_list[].encrypt_info.spade_a" and its JSON type.
// "[]" steps into every array element and "{}" into a string that itself
// holds a JSON object, as video_model does.
type Field struct {
	Path string
	Type string
}

// Shape lists the leaf fields of a JSON document, sorted by path. A path
// that appears with several types (in different array elements) is listed
// once per type.
func Shape(raw []byte) ([]Field, error) {
	var v any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	seen := map[Field]bool{}
	walk("", v, seen)
	out := make([]Field, 0, len(seen))
	for f := range seen {
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Type < out[j].Type
	})
	return out, nil
}

func walk(path string, v any, seen map[Field]bool) {
	switch x := v.(type) {
	case map[string]any:
		if len(x) == 0 {
			seen[Field{path, "object"}] = true
		}
		for k, child := range x {
			walk(join(path, k), child, seen)
		}
	case []any:
		if len(x) == 0 {
			seen[Field{path, "array"}] = true
		}
		for _, child := range x {
			walk(path+"[]", child, seen)
		}
	case string:
		if inner, ok := embeddedObject(x); ok {
			walk(path+"{}", inner, seen)
			return
		}
		seen[Field{path, "string"}] = true
	case json.Number:
		seen[Field{path, "number"}] = true
	case bool:
		seen[Field{path, "bool"}] = true
	case nil:
		seen[Field{path, "null"}] = true
	}
}

func join(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}

// embeddedObject decodes a string that holds a JSON object.
func embeddedObject(s string) (any, bool) {
	t := strings.TrimSpace(s)
	if !strings.HasPrefix(t, "{") {
		return nil, false
	}
	var v any
	dec := json.NewDecoder(strings.NewReader(t))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return nil, false
	}
	if _, ok := v.(map[string]any); !ok {
		return nil, false
	}
	return v, true
}

// Drift is one fixture field the live response no longer matches.
type Drift struct {
	Path string
	// Want is the fixture's type; Got lists the live types at that path,
	// empty when the path is gone.
	Want string
	Got  []string
}

func (d Drift) String() string {
	if len(d.Got) == 0 {
		return fmt.Sprintf("%s: missing (fixture has %s)", d.Path, d.Want)
	}
	return fmt.Sprintf("%s: type %s, fixture has %s", d.Path, strings.Join(d.Got, "|"), d.Want)
}

// Compare reports every fixture field whose path is missing from live or
// never carries the fixture's type there. An empty array or object in the
// fixture is satisfied by any value below that path, and null satisfies a
// string, number or bool: the parsers read it as the zero value, and which
// optional fields are set depends on the title or person the run lands on.
// Fields only the live response has are not drift: the parsers ignore them.
func Compare(fixture, live []Field) []Drift {
	liveTypes := map[string][]string{}
	for _, f := range live {
		liveTypes[f.Path] = append(liveTypes[f.Path], f.Type)
	}
	var drifts []Drift
	for _, f := range fixture {
		got := liveTypes[f.Path]
		if contains(got, f.Type) {
			continue
		}
		if isScalar(f.Type) && contains(got, "null") {
			continue
		}
		if (f.Type == "array" || f.Type == "object") && hasPrefix(live, f.Path) {
			continue
		}
		drifts = append(drifts, Drift{Path: f.Path, Want: f.Type, Got: dedupe(got)})
	}
	return drifts
}

func hasPrefix(fields []Field, path string) bool {
	for _, f := range fields {
		if strings.HasPrefix(f.Path, path+".") || strings.HasPrefix(f.Path, path+"[]") || strings.HasPrefix(f.Path, path+"{}") {
			return true
		}
	}
	return false
}

func isScalar(typ string) bool {
	return typ == "string" || typ == "number" || typ == "bool"
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func dedupe(list []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, x := range list {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	sort.Strings(out)
	return out
}
