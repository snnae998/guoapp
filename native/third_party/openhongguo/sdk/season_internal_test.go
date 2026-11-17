package sdk

import "testing"

func TestSeasonBaseTitleStripsMarkers(t *testing.T) {
	cases := map[string]string{
		"全球死考：刚下刑场你让我去考试第一季": "全球死考：刚下刑场你让我去考试",
		"Silent Court 第二季":   "Silent Court",
		"Some Show 3":        "Some Show",
		"Plain Title":        "Plain Title",
	}
	for in, want := range cases {
		if got := seasonBaseTitle(in); got != want {
			t.Fatalf("seasonBaseTitle(%q) = %q want %q", in, got, want)
		}
	}
}
