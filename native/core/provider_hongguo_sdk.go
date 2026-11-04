package core

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/beilunyang/openhongguo/adapters"
	"github.com/beilunyang/openhongguo/sdk"
	sdkmodel "github.com/beilunyang/openhongguo/sdk/model"
)

const hongguoSDKMaxDevices = 3

const hongguoSDKProfileName = "hongguo-sdk-device.json"

func (d *Downloader) hongguoSDKClient() (*sdk.Client, error) {
	d.hongguoSDKOnce.Do(func() {
		dir := strings.TrimSpace(d.cfg.dataDir)
		if dir == "" {
			dir = os.TempDir()
		}
		client, err := adapters.NewClient(filepath.Join(dir, hongguoSDKProfileName), sdk.Options{RetryAttempts: 3})
		if err != nil {
			d.hongguoSDKErr = fmt.Errorf("红果 SDK 初始化失败: %w", err)
			return
		}
		d.hongguoSDK = client
	})
	if d.hongguoSDK == nil {
		if d.hongguoSDKErr != nil {
			return nil, d.hongguoSDKErr
		}
		return nil, errors.New("红果 SDK 初始化失败")
	}
	return d.hongguoSDK, nil
}

func (d *Downloader) resolveHongguoSDKPlayback(ctx context.Context, episodeID string) (providerMedia, error) {
	if !hongguoNumericID.MatchString(episodeID) {
		return providerMedia{}, errors.New("红果视频 ID 无效")
	}
	client, err := d.hongguoSDKClient()
	if err != nil {
		return providerMedia{}, err
	}
	var pb *sdkmodel.Playback
	var lastErr error
	for attempt := 0; attempt < hongguoSDKMaxDevices; attempt++ {
		if attempt > 0 {
			d.hongguoSDKMu.Lock()
			resetErr := client.ResetDevice(ctx)
			d.hongguoSDKMu.Unlock()
			if resetErr != nil {
				return providerMedia{}, fmt.Errorf("红果 SDK 更换设备失败: %w", resetErr)
			}
		}
		reqCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
		pb, lastErr = client.ResolvePlayback(reqCtx, episodeID, sdk.PlaybackOptions{})
		cancel()
		if lastErr == nil {
			break
		}
		if err := ctx.Err(); err != nil {
			return providerMedia{}, err
		}
		if !errors.Is(lastErr, sdk.ErrAnonymousLimited) && !errors.Is(lastErr, sdk.ErrRateLimited) {
			break
		}
	}
	if lastErr != nil {
		return providerMedia{}, fmt.Errorf("红果 SDK 取流失败: %w", lastErr)
	}
	return hongguoSDKPlaybackToMedia(pb)
}

func hongguoSDKPlaybackToMedia(pb *sdkmodel.Playback) (providerMedia, error) {
	if pb == nil || len(pb.Streams) == 0 {
		return providerMedia{}, errors.New("红果 SDK 未返回可用流")
	}
	var cencKey []byte
	if pb.Protected {
		if key, err := sdk.ContentKey(pb); err == nil {
			if decoded, derr := hex.DecodeString(strings.TrimSpace(key.Hex)); derr == nil && len(decoded) > 0 {
				cencKey = decoded
			}
		}
	}
	var variants []providerMedia
	for _, s := range pb.Streams {
		mediaURL := strings.TrimSpace(s.URL)
		if mediaURL == "" || len(mediaURL) > 8192 || !isProviderHTTPMediaURL(mediaURL) {
			if backup := strings.TrimSpace(s.BackupURL); backup != "" && len(backup) <= 8192 && isProviderHTTPMediaURL(backup) {
				mediaURL = backup
			} else {
				continue
			}
		}
		variants = append(variants, providerMedia{
			URL:     mediaURL,
			Referer: "https://novel.snssdk.com/",
			CENCKey: cencKey,
			Quality: hongguoSDKStreamQuality(s),
		})
	}
	if len(variants) == 0 {
		return providerMedia{}, errors.New("红果 SDK 未返回可用媒体地址")
	}
	selected := variants[0]
	for _, v := range variants[1:] {
		if v.Quality > selected.Quality {
			selected = v
		}
	}
	selected.Variants = variants
	return selected, nil
}

func hongguoSDKStreamQuality(s sdkmodel.Stream) int {
	if s.Height > 0 {
		return s.Height
	}
	if num := hongguoQualityNumber.FindString(s.Definition); num != "" {
		if q, err := strconv.Atoi(num); err == nil {
			return q
		}
	}
	return 0
}
