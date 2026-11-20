// Package memstore 只在内存里保存匿名设备档案（sdk.ProfileStore），进程退出后丢失。
// 适合测试，以及不想在磁盘上留下设备的场景。
package memstore

import (
	"context"
	"sync"

	"github.com/beilunyang/openhongguo/sdk"
)

// Store 在内存里保存一份档案，可以并发使用。
type Store struct {
	mu      sync.Mutex
	profile *sdk.DeviceProfile
}

// New 返回一个空的 Store。
func New() *Store { return &Store{} }

// Load 返回档案的副本；没有档案时返回 sdk.ErrProfileNotFound。
func (s *Store) Load(ctx context.Context) (*sdk.DeviceProfile, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.profile == nil {
		return nil, sdk.ErrProfileNotFound
	}
	cp := *s.profile
	return &cp, nil
}

// Save 保存档案的副本。
func (s *Store) Save(ctx context.Context, profile *sdk.DeviceProfile) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if profile == nil {
		return sdk.ErrInvalidOptions
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *profile
	s.profile = &cp
	return nil
}

// Reset 丢弃档案。
func (s *Store) Reset(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.profile = nil
	return nil
}
