// Command docgen 根据 sdk 的 Go 源码和注释，生成各语言 SDK 共用的文档：
// README 里的方法表和错误表，以及类型参考 docs/types.md。
//
// 方法和错误的一句话说明取自 Go 注释的首句，方法名和错误码取自
// bindings/internal/dispatch，类型取自 bindings/internal/schema。README 里
// 生成的部分夹在 <!-- gen:名字 --> 和 <!-- /gen:名字 --> 之间，其余部分
// 手写，不受影响。
//
// 用法：go run ./bindings/docgen（在仓库根目录运行）
package main

import (
	"fmt"
	"go/ast"
	"go/doc"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/beilunyang/openhongguo/bindings/internal/schema"
)

// regions 列出每个 README 里要生成的区块。
var regions = map[string][]string{
	"README.md":                  {"methods-go", "errors-go"},
	"bindings/node/README.md":    {"methods", "errors"},
	"bindings/mobile/README.md":  {"methods", "errors"},
	"bindings/flutter/README.md": {"methods", "errors"},
}

const typesFile = "docs/types.md"

func main() {
	out, err := generate(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, "docgen:", err)
		os.Exit(1)
	}
	for path, content := range out {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "docgen:", err)
			os.Exit(1)
		}
	}
}

// generate 返回仓库根目录 root 对应的输出，键是相对 root 的路径。
func generate(root string) (map[string]string, error) {
	api, err := loadAPI(filepath.Join(root, "sdk"))
	if err != nil {
		return nil, err
	}
	s, err := schema.Load(root)
	if err != nil {
		return nil, err
	}
	blocks := map[string]func() (string, error){
		"methods-go": func() (string, error) { return methodTables(api, true) },
		"methods":    func() (string, error) { return methodTables(api, false) },
		"errors-go":  func() (string, error) { return errorTable(api, true) },
		"errors":     func() (string, error) { return errorTable(api, false) },
	}
	out := map[string]string{}
	for path, names := range regions {
		raw, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			return nil, err
		}
		text := string(raw)
		for _, name := range names {
			body, err := blocks[name]()
			if err != nil {
				return nil, err
			}
			if text, err = fill(text, name, body); err != nil {
				return nil, fmt.Errorf("%s: %w", path, err)
			}
		}
		out[filepath.FromSlash(path)] = text
	}
	out[filepath.FromSlash(typesFile)] = typesDoc(s)
	return out, nil
}

// fill 把 text 里 name 区块的内容换成 body。
func fill(text, name, body string) (string, error) {
	begin := "<!-- gen:" + name + " -->"
	end := "<!-- /gen:" + name + " -->"
	i := strings.Index(text, begin)
	j := strings.Index(text, end)
	if i < 0 || j < i {
		return "", fmt.Errorf("region %s not found", name)
	}
	return text[:i+len(begin)] + "\n" + note + "\n\n" + body + text[j:], nil
}

const note = "<!-- 由 go run ./bindings/docgen 生成，请修改 sdk 的 Go 注释，不要手工修改。 -->"

// api 是 sdk 包里文档要用到的声明。
type api struct {
	pkg   *doc.Package
	funcs map[string]*doc.Func // Client 的方法和包级函数，按名字
	vars  map[string]string    // 哨兵错误的注释，按名字
	order []string             // 哨兵错误的声明顺序
}

func loadAPI(dir string) (*api, error) {
	fset := token.NewFileSet()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []*ast.File
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, n), nil, parser.ParseComments)
		if err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	pkg, err := doc.NewFromFiles(fset, files, "github.com/beilunyang/openhongguo/sdk")
	if err != nil {
		return nil, err
	}
	a := &api{pkg: pkg, funcs: map[string]*doc.Func{}, vars: map[string]string{}}
	for _, f := range pkg.Funcs {
		a.funcs[f.Name] = f
	}
	for _, t := range pkg.Types {
		for _, f := range t.Funcs {
			a.funcs[f.Name] = f
		}
		if t.Name == "Client" {
			for _, m := range t.Methods {
				a.funcs[m.Name] = m
			}
		}
	}
	for _, v := range pkg.Vars {
		for _, spec := range v.Decl.Specs {
			vs := spec.(*ast.ValueSpec)
			for _, n := range vs.Names {
				if !strings.HasPrefix(n.Name, "Err") {
					continue
				}
				a.vars[n.Name] = vs.Doc.Text()
				a.order = append(a.order, n.Name)
			}
		}
	}
	return a, nil
}

// synopsis 取注释的首句，去掉开头的标识符 name 和结尾的句号，用作表格的一格。
func (a *api) synopsis(name, text string) string {
	s := strings.TrimSpace(strings.TrimPrefix(a.pkg.Synopsis(text), name))
	return strings.TrimSuffix(s, "。")
}

// cell 让文字可以放进表格的一格：合并成一行，转义竖线。
func cell(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	return strings.ReplaceAll(cjkJoin(s), "|", `\|`)
}

// cjkJoin 去掉两个中文字符之间、以及全角标点前后的空格。注释换行处的
// 中文，合并成一行时会被插入空格；中文和英文之间的空格保留。
func cjkJoin(s string) string {
	r := []rune(s)
	var b strings.Builder
	for i, c := range r {
		if c == ' ' && i > 0 && i+1 < len(r) && (wide(r[i-1]) && wide(r[i+1]) || punct(r[i-1]) || punct(r[i+1])) {
			continue
		}
		b.WriteRune(c)
	}
	return b.String()
}

// wide 报告 c 是否是中日韩文字或全角标点。
func wide(c rune) bool { return c >= 0x2E80 }

// punct 报告 c 是否是全角标点。
func punct(c rune) bool { return c >= 0x3000 && c <= 0x303F || c >= 0xFF00 && c <= 0xFFEF }
