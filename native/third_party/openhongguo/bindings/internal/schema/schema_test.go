package schema

import (
	"reflect"
	"testing"
)

func TestWords(t *testing.T) {
	for in, want := range map[string][]string{
		"CoverURL":      {"Cover", "URL"},
		"PPTRecommend":  {"PPT", "Recommend"},
		"PUGC":          {"PUGC"},
		"UGCVideo":      {"UGC", "Video"},
		"SeriesUpdated": {"Series", "Updated"},
	} {
		if got := Words(in); !reflect.DeepEqual(got, want) {
			t.Errorf("Words(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEnumPrefix(t *testing.T) {
	consts := func(names ...string) []Const {
		var out []Const
		for _, n := range names {
			out = append(out, Const{GoName: n})
		}
		return out
	}
	tests := []struct {
		names []string
		want  string
	}{
		{[]string{"ContentUnknown", "ContentShortSeries", "ContentPUGC"}, "Content"},
		{[]string{"SeriesUpdating", "SeriesUpdatedToday"}, "Series"},
		// 去掉前缀后不能什么都不剩。
		{[]string{"RankNew", "Rank"}, ""},
		{[]string{"Only"}, ""},
	}
	for _, tt := range tests {
		if got := EnumPrefix(consts(tt.names...)); got != tt.want {
			t.Errorf("EnumPrefix(%v) = %q, want %q", tt.names, got, tt.want)
		}
	}
}

func TestLoad(t *testing.T) {
	s, err := Load("../../..")
	if err != nil {
		t.Fatal(err)
	}
	series, ok := s.Lookup("Series")
	if !ok || !series.Struct || len(series.Embeds) != 1 {
		t.Fatalf("Series = %+v", series)
	}
	// 嵌入的 SeriesSummary 字段排在前面。
	if f := s.AllFields(series)[0]; f.JSONName != "id" {
		t.Errorf("first field = %+v", f)
	}
	ct, _ := s.Lookup("ContentType")
	if ct.Struct || ct.Underlying.Kind != Int || len(ct.Consts) == 0 {
		t.Errorf("ContentType = %+v", ct)
	}
	if len(s.Queries) != len(QueryTypes) || s.Queries[1].Fields[1].JSONName != "pageSize" {
		t.Errorf("queries = %+v", s.Queries)
	}
}
