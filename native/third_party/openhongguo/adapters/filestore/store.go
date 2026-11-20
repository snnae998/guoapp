// Package filestore 把匿名设备档案保存成一个 JSON 文件（sdk.ProfileStore）。
package filestore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/beilunyang/openhongguo/sdk"
)

// Store 把设备档案存在一个文件里，权限 0600，可以并发使用。
// 写入先写临时文件再改名，中途失败不会留下半个文件；路径是符号链接时拒绝读写。
type Store struct {
	path string
	mu   sync.Mutex
}

// New 返回保存在 path 的 Store。目录在首次保存时创建，权限 0700。
func New(path string) *Store { return &Store{path: path} }

// Load 读取档案；文件不存在时返回 sdk.ErrProfileNotFound。
func (s *Store) Load(ctx context.Context) (*sdk.DeviceProfile, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := rejectSymlink(s.path); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, sdk.ErrProfileNotFound
	}
	if err != nil {
		return nil, err
	}
	var p sdk.DeviceProfile
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// Save 保存档案。
func (s *Store) Save(ctx context.Context, p *sdk.DeviceProfile) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if p == nil {
		return sdk.ErrInvalidOptions
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := rejectSymlink(s.path); err != nil {
		return err
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".ohg-profile-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return err
	}
	if err := json.NewEncoder(tmp).Encode(p); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(name, s.path); err != nil {
		return err
	}
	return os.Chmod(s.path, 0600)
}

// Reset 删除档案文件；文件不存在时不报错。
func (s *Store) Reset(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := rejectSymlink(s.path); err != nil {
		return err
	}
	err := os.Remove(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func rejectSymlink(path string) error {
	if path == "" {
		return sdk.ErrInvalidOptions
	}
	info, err := os.Lstat(path)
	if err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("openhongguo: symlink profile path")
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
