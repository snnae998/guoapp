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

const hongguoSDKDeviceResets = 5

const hongguoSDKPlaybackRounds = 3

const hongguoSDKProfileName = "hongguo-sdk-device.json"

func (d *Downloader) hongguoSDKClient() (*sdk.Client, error) {
	d.hongguoSDKOnce.Do(func() {
		dir := strings.TrimSpace(d.cfg.dataDir)
		if dir == "" {
			dir = os.TempDir()
		}
		client, err := adapters.NewClient(filepath.Join(dir, hongguoSDKProfileName), sdk.Options{RetryAttempts: 4, DeviceResets: sdk.Int(hongguoSDKDeviceResets)})
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
	reqCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	var lastErr error
	for round := 0; round < hongguoSDKPlaybackRounds; round++ {
		if round > 0 {
			if err := client.ResetDevice(reqCtx); err != nil {
				break
			}
			timer := time.NewTimer(time.Duration(round) * 600 * time.Millisecond)
			select {
			case <-reqCtx.Done():
				timer.Stop()
				return providerMedia{}, reqCtx.Err()
			case <-timer.C:
			}
		}
		pb, err := client.ResolvePlayback(reqCtx, episodeID, sdk.PlaybackOptions{})
		if err != nil && (!errors.Is(err, sdk.ErrDRMProtected) || pb == nil) {
			lastErr = err
			if !hongguoSDKDeviceFailure(err) {
				break
			}
			continue
		}
		media, convErr := hongguoSDKPlaybackToMedia(pb)
		if convErr == nil {
			return media, nil
		}
		lastErr = convErr
	}
	if lastErr != nil {
		return providerMedia{}, fmt.Errorf("红果 SDK 取流失败: %w", lastErr)
	}
	return providerMedia{}, errors.New("红果 SDK 取流失败")
}

func hongguoSDKDeviceFailure(err error) bool {
	return errors.Is(err, sdk.ErrAnonymousLimited) || errors.Is(err, sdk.ErrDeviceRejected)
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
