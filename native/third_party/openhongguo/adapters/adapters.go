// Package adapters 提供 SDK 的默认实现，以及一步创建客户端的 NewClient。
//
// 子包可以单独使用：httptransport 发送网络请求，filestore 把设备档案存成
// 文件，memstore 只在内存里保存设备档案。
package adapters

import (
	"fmt"

	"github.com/beilunyang/openhongguo/adapters/filestore"
	"github.com/beilunyang/openhongguo/adapters/httptransport"
	"github.com/beilunyang/openhongguo/sdk"
)

// NewClient 用默认实现创建客户端：opts.Transport 为空时用 httptransport，
// opts.ProfileStore 为空时把设备档案保存在 profilePath（filestore）。
// 其余设置照常取默认值，见 sdk.Options。
//
// opts.ProfileStore 为空而 profilePath 也为空时，返回 sdk.ErrInvalidOptions。
func NewClient(profilePath string, opts sdk.Options) (*sdk.Client, error) {
	if opts.Transport == nil {
		opts.Transport = httptransport.New()
	}
	if opts.ProfileStore == nil {
		if profilePath == "" {
			return nil, fmt.Errorf("%w: profile path is empty", sdk.ErrInvalidOptions)
		}
		opts.ProfileStore = filestore.New(profilePath)
	}
	return sdk.NewClient(opts)
}
