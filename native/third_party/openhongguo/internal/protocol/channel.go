package protocol

import (
	"fmt"
	"strconv"
	"strings"
)

// DiscoverChannel is one anonymous bookmall channel.
// RequestTab is the tab_type sent on bookmall/tab/v.
// ResponseTab is the tab_type whose cards are kept.
// BottomTab is bottom_tab_type. Empty means the short-drama mall (7).
type DiscoverChannel struct {
	Name        string
	RequestTab  string
	ResponseTab int
	BottomTab   string
}

// ResolveDiscoverChannel maps a CLI channel name to the bookmall tab.
// 推荐 keeps the verified open (tab_type -1) and reads video_feed (16).
// 漫剧 is motion_comic_feed (36). 真人剧 is real_person_feed (39).
// Those three stay on the short-drama mall, bottom_tab_type 7.
// The home bar is BookStore (0), except 社区 which is Community (10).
// Home 漫剧 is comic_series (32) and home 真人剧 is real_person_series (38).
// Those names stay distinct from the mall feeds.
func ResolveDiscoverChannel(name string) (DiscoverChannel, error) {
	switch strings.TrimSpace(name) {
	case "", "推荐", "recommend":
		return DiscoverChannel{Name: "推荐", RequestTab: "-1", ResponseTab: 16, BottomTab: "7"}, nil
	case "漫剧", "motion":
		return DiscoverChannel{Name: "漫剧", RequestTab: "36", ResponseTab: 36, BottomTab: "7"}, nil
	case "真人剧", "live":
		return DiscoverChannel{Name: "真人剧", RequestTab: "39", ResponseTab: 39, BottomTab: "7"}, nil
	case "找剧":
		return DiscoverChannel{Name: "找剧", RequestTab: "8", ResponseTab: 8, BottomTab: "0"}, nil
	case "首页真人剧":
		return DiscoverChannel{Name: "首页真人剧", RequestTab: "38", ResponseTab: 38, BottomTab: "0"}, nil
	case "首页漫剧":
		return DiscoverChannel{Name: "首页漫剧", RequestTab: "32", ResponseTab: 32, BottomTab: "0"}, nil
	case "电影":
		return DiscoverChannel{Name: "电影", RequestTab: "14", ResponseTab: 14, BottomTab: "0"}, nil
	case "听书":
		return DiscoverChannel{Name: "听书", RequestTab: "5", ResponseTab: 5, BottomTab: "0"}, nil
	case "小说":
		return DiscoverChannel{Name: "小说", RequestTab: "2", ResponseTab: 2, BottomTab: "0"}, nil
	case "社区":
		return DiscoverChannel{Name: "社区", RequestTab: "33", ResponseTab: 33, BottomTab: "10"}, nil
	case "漫画":
		return DiscoverChannel{Name: "漫画", RequestTab: "9", ResponseTab: 9, BottomTab: "0"}, nil
	case "大视听":
		return DiscoverChannel{Name: "大视听", RequestTab: "19", ResponseTab: 19, BottomTab: "0"}, nil
	case "经典":
		return DiscoverChannel{Name: "经典", RequestTab: "11", ResponseTab: 11, BottomTab: "0"}, nil
	case "知识":
		return DiscoverChannel{Name: "知识", RequestTab: "12", ResponseTab: 12, BottomTab: "0"}, nil
	default:
		return DiscoverChannel{}, fmt.Errorf("unknown discover channel %q", name)
	}
}

// ResolveSearchTab maps a CLI search tab to SearchTabType.
// An empty tab is 综合 (ShortPlay, 11). Comic and dynamic-comic tab types
// return the tab bar with empty rows on this guest call.
func ResolveSearchTab(name string) (string, error) {
	switch strings.TrimSpace(name) {
	case "", "短剧", "short", "综合":
		return SearchTabShortPlay, nil
	case "剧集":
		return SearchTabSeriesVideo, nil
	case "视频":
		return SearchTabPugcVideo, nil
	case "讨论":
		return SearchTabDiscussion, nil
	case "小说":
		return SearchTabCommon, nil
	case "用户":
		return SearchTabHgUser, nil
	case "听书":
		return SearchTabVoice, nil
	default:
		return "", fmt.Errorf("unknown search tab %q", name)
	}
}

// SearchOffset reads a search cursor. An empty cursor is the first page.
// The cursor is the next request's offset query value.
func SearchOffset(cursor string) (int, error) {
	if strings.TrimSpace(cursor) == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(cursor)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("search cursor is not verified")
	}
	return n, nil
}
