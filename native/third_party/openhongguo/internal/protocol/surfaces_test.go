package protocol

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testdata/anonymous-surfaces.json 是从 APK 整理出的匿名接口索引（路径和请求字段），
// 摘自本机的 docs/rea/anonymous-surfaces.json，去掉了证据引用。
func TestShippedPathsAreInAnonymousSurfaceEvidence(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "anonymous-surfaces.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Package     string `json:"package"`
		VersionName string `json:"version_name"`
		VersionCode string `json:"version_code"`
		Surfaces    []struct {
			ID            string   `json:"id"`
			Path          string   `json:"path"`
			RequestFields []string `json:"request_fields"`
		} `json:"surfaces"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Package != Package || doc.VersionName != VersionName || doc.VersionCode != VersionCode {
		t.Fatalf("package evidence = %s %s %s", doc.Package, doc.VersionName, doc.VersionCode)
	}
	byID := map[string]struct {
		path   string
		fields map[string]bool
	}{}
	for _, surface := range doc.Surfaces {
		fields := make(map[string]bool, len(surface.RequestFields))
		for _, field := range surface.RequestFields {
			fields[field] = true
		}
		byID[surface.ID] = struct {
			path   string
			fields map[string]bool
		}{path: surface.Path, fields: fields}
	}

	seriesBody, err := SeriesBody("7100000000000101")
	if err != nil {
		t.Fatal(err)
	}
	var series map[string]any
	if err := json.Unmarshal(seriesBody, &series); err != nil {
		t.Fatal(err)
	}
	seriesKeys := make([]string, 0, 8)
	for key, value := range series {
		seriesKeys = append(seriesKeys, key)
		nested, ok := value.(map[string]any)
		if !ok {
			continue
		}
		for child := range nested {
			seriesKeys = append(seriesKeys, child)
		}
	}

	checks := []struct {
		id   string
		path string
		keys []string
	}{
		{id: "guest_registration", path: PathRegister, keys: []string{"req_id", "tt_data", "magic_tag", "_gen_time"}},
		{id: "discovery", path: PathDiscover, keys: queryKeys(DiscoverQuery("1", "2", 3))},
		{id: "discovery_unlimited", path: PathCellChange, keys: []string{"cell_id", "offset", "limit", "change_type", "tab_type", "show_type", "client_req_type", "session_id", "client_template"}},
		{id: "search", path: PathSearch, keys: queryKeys(SearchQuery("短剧", "1", "2", 3, 20))},
		{id: "series_detail", path: PathSeries, keys: seriesKeys},
		{id: "episode_list", path: PathSeries, keys: []string{"series_id"}},
	}
	for _, check := range checks {
		got, ok := byID[check.id]
		if !ok {
			t.Fatalf("missing surface %s", check.id)
		}
		if got.path != check.path {
			t.Fatalf("%s path = %s, client path = %s", check.id, got.path, check.path)
		}
		for _, key := range check.keys {
			if !got.fields[key] {
				t.Fatalf("%s sends %s, which is absent from the evidence index", check.id, key)
			}
		}
	}
	if byID["playback_classification"].path != PathPlayback {
		t.Fatalf("playback path = %s", byID["playback_classification"].path)
	}
	if !strings.Contains(string(raw), "/novel/player/video_detail/v1/") || !strings.Contains(string(raw), "/reading/bookapi/bookmall/tab/v") {
		t.Fatal("evidence index dropped a guest read path")
	}
}

func queryKeys(q url.Values) []string {
	keys := make([]string, 0, len(q))
	for key := range q {
		keys = append(keys, key)
	}
	return keys
}
