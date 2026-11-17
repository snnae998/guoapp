package sdk

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"time"

	"github.com/beilunyang/openhongguo/sdk/model"
)

// DeviceProfile 是本地生成的匿名安装。服务端分配的 id 在注册返回之前保持为空。
// CLI 从不打印 DeviceToken。
type DeviceProfile struct {
	// CDID、OpenUDID、ClientUDID、OAID 和 ReqID 是随机安装标识符，
	// 由 App 在本地生成，注册时发送。
	CDID       string `json:"cdid"`
	OpenUDID   string `json:"openudid"`
	ClientUDID string `json:"clientudid"`
	OAID       string `json:"oaid_id"`
	ReqID      string `json:"req_id"`
	// DeviceID 和 InstallID 由注册分配，每次请求都会发送；
	// DeviceToken 随它们一起下发。
	DeviceID    string `json:"device_id_str,omitempty"`
	InstallID   string `json:"install_id_str,omitempty"`
	DeviceToken string `json:"device_token,omitempty"`
	// TTReq、StoreRegion 和 StoreRegionSrc 是服务端设置的 cookie，
	// 后续请求会原样带上。
	TTReq          string `json:"ttreq,omitempty"`
	StoreRegion    string `json:"store_region,omitempty"`
	StoreRegionSrc string `json:"store_region_src,omitempty"`
	// CreatedAt 是档案的生成时间（UTC）。
	CreatedAt time.Time `json:"created_at"`
	// Device 是档案注册时使用的手机，每次请求都出示它。
	// 没有该字段的档案在首次使用时采用客户端的设备。
	Device *model.Device `json:"device,omitempty"`
}

// ProfileStore 持久化保存一个匿名档案。
type ProfileStore interface {
	Load(ctx context.Context) (*DeviceProfile, error)
	Save(ctx context.Context, profile *DeviceProfile) error
	Reset(ctx context.Context) error
}

// NewAnonymousDeviceProfile 用密码学安全的随机数填充身份字段。
// entropy 为 nil 时使用 crypto/rand；clock 为 nil 时使用 time.Now。
func NewAnonymousDeviceProfile(entropy io.Reader, clock func() time.Time) (*DeviceProfile, error) {
	if entropy == nil {
		entropy = rand.Reader
	}
	if clock == nil {
		clock = time.Now
	}
	var raw [72]byte
	if _, err := io.ReadFull(entropy, raw[:]); err != nil {
		return nil, err
	}
	return &DeviceProfile{
		CDID:       uuid(raw[0:16]),
		OpenUDID:   hex.EncodeToString(raw[16:24]),
		ClientUDID: uuid(raw[24:40]),
		OAID:       uuid(raw[40:56]),
		ReqID:      uuid(raw[56:72]),
		CreatedAt:  clock().UTC(),
	}, nil
}

func uuid(b []byte) string {
	var id [16]byte
	copy(id[:], b)
	id[6] = id[6]&0x0f | 0x40
	id[8] = id[8]&0x3f | 0x80
	var s [36]byte
	hex.Encode(s[0:8], id[0:4])
	s[8] = '-'
	hex.Encode(s[9:13], id[4:6])
	s[13] = '-'
	hex.Encode(s[14:18], id[6:8])
	s[18] = '-'
	hex.Encode(s[19:23], id[8:10])
	s[23] = '-'
	hex.Encode(s[24:36], id[10:16])
	return string(s[:])
}
