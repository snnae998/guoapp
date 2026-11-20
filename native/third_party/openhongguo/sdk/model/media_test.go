package model

import "testing"

func TestSelectOrdersByExplicitHeightThenBitrate(t *testing.T) {
	p := &Playback{Streams: []Stream{
		{Definition: "720p", Height: 720, Bitrate: 900, URL: "https://media.invalid/720-low.mp4"},
		{Definition: "720p", Height: 720, Bitrate: 1800, URL: "https://media.invalid/720-high.mp4"},
		{Definition: "1080p", Height: 1080, Bitrate: 3000, URL: "https://media.invalid/1080.mp4"},
		{Definition: "360p", Height: 360, URL: " "},
	}}
	best, err := p.Select("best")
	if err != nil || best.URL != "https://media.invalid/1080.mp4" {
		t.Fatalf("best = %+v err = %v", best, err)
	}
	worst, err := p.Select("worst")
	if err != nil || worst.URL != "https://media.invalid/720-low.mp4" {
		t.Fatalf("worst = %+v err = %v", worst, err)
	}
	// An empty URL is never selected, so the 360p row is skipped.
	if _, err := p.Select("360p"); err == nil {
		t.Fatal("stream without a URL was selected")
	}
	if _, err := p.Select("1080p"); err != nil {
		t.Fatalf("exact definition err = %v", err)
	}
}

func TestSelectReportsAvailableDefinitions(t *testing.T) {
	p := &Playback{Streams: []Stream{{Definition: "540p", URL: "https://media.invalid/a.mp4"}, {Quality: "normal", URL: "https://media.invalid/b.mp4"}}}
	_, err := p.Select("2160p")
	if err == nil {
		t.Fatal("missing quality was accepted")
	}
	if got := err.Error(); got != `quality "2160p" is not offered (available: normal 540p)` {
		t.Fatalf("err = %s", got)
	}
}
