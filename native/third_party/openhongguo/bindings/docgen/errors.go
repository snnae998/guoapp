package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/beilunyang/openhongguo/bindings/internal/dispatch"
	"github.com/beilunyang/openhongguo/bindings/internal/schema"
	"github.com/beilunyang/openhongguo/sdk"
)

// errorTable 生成错误表，行的顺序与 dispatch.ErrorCodes 相同。goNames 为真时
// 同时列出 Go 的哨兵错误（根目录 README）。
func errorTable(a *api, goNames bool) (string, error) {
	byCode := map[string]string{}
	for _, name := range a.order {
		byCode[codeOf(name)] = name
	}
	var b strings.Builder
	if goNames {
		b.WriteString("| Go | 错误码 | 含义 | 可重试 |\n|---|---|---|---|\n")
	} else {
		b.WriteString("| 错误码 | 含义 | 可重试 |\n|---|---|---|\n")
	}
	var cancelled string
	for _, e := range dispatch.ErrorCodes {
		if errors.Is(e.Err, context.Canceled) {
			cancelled = e.Code
			continue
		}
		name, ok := byCode[e.Code]
		if !ok {
			return "", fmt.Errorf("error code %s matches no sentinel in sdk", e.Code)
		}
		desc := cell(strings.TrimPrefix(a.synopsis(name, a.vars[name]), "表示"))
		if goNames {
			fmt.Fprintf(&b, "| `%s` | `%s` | %s | %s |\n", name, e.Code, desc, yesNo(sdk.Retryable(e.Err)))
		} else {
			fmt.Fprintf(&b, "| `%s` | %s | %s |\n", e.Code, desc, yesNo(sdk.Retryable(e.Err)))
		}
	}
	if !goNames && cancelled != "" {
		fmt.Fprintf(&b, "| `%s` | 调用被取消 | 否 |\n", cancelled)
	}
	return b.String(), nil
}

// codeOf 是哨兵错误 name 的错误码：ErrDRMProtected → DRM_PROTECTED。
func codeOf(name string) string {
	return strings.ToUpper(strings.Join(schema.Words(strings.TrimPrefix(name, "Err")), "_"))
}

func yesNo(v bool) string {
	if v {
		return "是"
	}
	return "否"
}
