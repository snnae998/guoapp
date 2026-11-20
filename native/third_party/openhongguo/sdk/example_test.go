package sdk_test

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/beilunyang/openhongguo/adapters"
	"github.com/beilunyang/openhongguo/adapters/filestore"
	"github.com/beilunyang/openhongguo/adapters/httptransport"
	"github.com/beilunyang/openhongguo/sdk"
)

// newClient 是各示例共用的客户端：使用默认的 HTTPS transport，
// 匿名设备保存在 anonymous.json 中。
func newClient() *sdk.Client {
	client, err := adapters.NewClient("anonymous.json", sdk.Options{RetryAttempts: 3})
	if err != nil {
		log.Fatal(err)
	}
	return client
}

func ExampleNewClient() {
	client, err := sdk.NewClient(sdk.Options{
		Transport:    httptransport.New(),
		ProfileStore: filestore.New("anonymous.json"),
	})
	if err != nil {
		log.Fatal(err)
	}
	_ = client
}

func ExampleClient_Search() {
	client := newClient()
	page, err := client.Search(context.Background(), "全球死考", sdk.SearchOptions{})
	if err != nil {
		log.Fatal(err)
	}
	for _, s := range page.Items {
		fmt.Println(s.ID, s.Title, s.EpisodeCount)
	}
}

func ExampleClient_DiscoverPager() {
	client := newClient()
	pager := client.DiscoverPager(sdk.DiscoverOptions{Channel: "漫剧"})
	for i := 0; i < 3; i++ {
		items, more, err := pager.Next(context.Background())
		if err != nil {
			log.Fatal(err)
		}
		for _, s := range items {
			fmt.Println(s.Title)
		}
		if !more {
			break
		}
	}
}

func ExampleClient_ResolvePlayback() {
	client := newClient()
	ctx := context.Background()
	episodes, err := client.ListEpisodes(ctx, "7675400266065136665", sdk.EpisodeOptions{})
	if err != nil {
		log.Fatal(err)
	}
	pb, err := client.ResolvePlayback(ctx, episodes.Items[0].ID, sdk.PlaybackOptions{})
	if err != nil && !errors.Is(err, sdk.ErrDRMProtected) {
		log.Fatal(err)
	}
	stream, err := pb.Select("720p")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(stream.URL)
	if pb.Protected {
		// 加密剧集还需要它的内容密钥。
		key, err := sdk.ContentKey(pb)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(key.Hex)
	}
}
