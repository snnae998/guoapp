package cliapp

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/beilunyang/openhongguo/internal/history"
	mediapkg "github.com/beilunyang/openhongguo/internal/media"
	"github.com/beilunyang/openhongguo/sdk"
	"github.com/beilunyang/openhongguo/sdk/model"
)

func epURL(n int) string {
	return "https://media.invalid/710000000000020" + string(rune('0'+n)) + ".mp4"
}

func searchResults() []model.SeriesSummary {
	return []model.SeriesSummary{
		{ID: "7100000000000999", Title: "全球死考第二季", EpisodeCount: 9},
		{ID: "7100000000000101", Title: "全球死考", EpisodeCount: 3},
	}
}

func TestPlayByTitlePrefersExactMatch(t *testing.T) {
	svc := &seqService{playService: playService{episodes: threeEpisodes()}, results: searchResults()}
	media := &fakeMedia{}
	var errOut bytes.Buffer
	deps := Dependencies{Service: svc, Media: media, Stdout: io.Discard, Stderr: &errOut}
	if code := Run(context.Background(), []string{"play", "全球死考"}, deps); code != ExitOK {
		t.Fatalf("exit %d (%s)", code, errOut.String())
	}
	if svc.query != "全球死考" || media.streamURL != epURL(1) {
		t.Fatalf("query = %q played = %q", svc.query, media.streamURL)
	}
	if !strings.Contains(errOut.String(), "全球死考 (7100000000000101, 3 episodes)") {
		t.Fatalf("stderr = %q", errOut.String())
	}
	svc.results = nil
	if code := Run(context.Background(), []string{"play", "nothing"}, deps); code != ExitFailure {
		t.Fatalf("no match exit %d", code)
	}
}

func TestPlayPickReadsChoice(t *testing.T) {
	svc := &seqService{playService: playService{episodes: threeEpisodes()}, results: searchResults()}
	media := &fakeMedia{}
	var errOut bytes.Buffer
	deps := Dependencies{Service: svc, Media: media, Stdin: strings.NewReader("2\n"), Stdout: io.Discard, Stderr: &errOut}
	if code := Run(context.Background(), []string{"play", "全球", "--pick"}, deps); code != ExitOK {
		t.Fatalf("exit %d (%s)", code, errOut.String())
	}
	if !strings.Contains(errOut.String(), " 1) 全球死考第二季 (9 episodes)") || media.streamURL != epURL(1) {
		t.Fatalf("stderr = %q played = %q", errOut.String(), media.streamURL)
	}
	deps.Stdin = strings.NewReader("7\n")
	if code := Run(context.Background(), []string{"play", "全球", "--pick"}, deps); code != ExitUsage {
		t.Fatalf("bad choice exit %d", code)
	}
}

func TestPlayJumpKeysSwitchEpisodes(t *testing.T) {
	svc := &seqService{playService: playService{episodes: threeEpisodes()}}
	media := &fakeMedia{
		streamResults: []mediapkg.Result{{Jump: mediapkg.JumpNext}, {Jump: mediapkg.JumpNext}, {Jump: mediapkg.JumpPrevious}},
		streamErrs:    []error{nil, nil, nil, mediapkg.ErrStopped},
	}
	deps := Dependencies{Service: svc, Media: media, Stdout: io.Discard, Stderr: io.Discard}
	if code := Run(context.Background(), []string{"play", "7100000000000101"}, deps); code != ExitOK {
		t.Fatalf("exit %d", code)
	}
	want := []string{epURL(1), epURL(2), epURL(3), epURL(2)}
	if strings.Join(media.streamed, " ") != strings.Join(want, " ") {
		t.Fatalf("streamed = %q", media.streamed)
	}
}

func TestPlayAutoplayPrefetchesEachEpisodeOnce(t *testing.T) {
	svc := &seqService{playService: playService{episodes: threeEpisodes()}}
	deps := Dependencies{Service: svc, Media: &fakeMedia{}, Stdout: io.Discard, Stderr: io.Discard}
	if code := Run(context.Background(), []string{"play", "7100000000000101", "--autoplay"}, deps); code != ExitOK {
		t.Fatalf("exit %d", code)
	}
	if len(svc.resolved) != 3 {
		t.Fatalf("resolved = %q", svc.resolved)
	}
}

func TestPlayRemembersPositionAndResumesThere(t *testing.T) {
	media := &fakeMedia{streamResults: []mediapkg.Result{{Position: 61.7}}, streamErrs: []error{mediapkg.ErrStopped}}
	deps, store, _ := historyDeps(t, media)
	ctx := context.Background()
	if code := Run(ctx, []string{"play", "7100000000000101", "--episode", "2"}, deps); code != ExitOK {
		t.Fatalf("exit %d", code)
	}
	e, _, _ := store.Get(ctx, "7100000000000101")
	if e.Episode != 2 || e.Finished || e.Position != 61.7 || e.Title != "剧/名" {
		t.Fatalf("entry = %+v", e)
	}
	media.streamed, media.streamErrs, media.streamResults = nil, nil, nil
	if code := Run(ctx, []string{"continue"}, deps); code != ExitOK {
		t.Fatalf("continue exit %d", code)
	}
	if media.streamURL != epURL(2) || media.streamStart != 58 {
		t.Fatalf("resumed %q at %d", media.streamURL, media.streamStart)
	}
}

func TestContinueWithoutHistory(t *testing.T) {
	deps, _, _ := historyDeps(t, &fakeMedia{})
	if code := Run(context.Background(), []string{"continue"}, deps); code != ExitUsage {
		t.Fatalf("exit %d", code)
	}
}

func TestPlayAutoplaySkipsUnavailableEpisode(t *testing.T) {
	svc := &seqService{playService: playService{episodes: threeEpisodes()},
		fail: map[string]error{"7100000000000202": sdk.ErrAnonymousLimited}}
	media := &fakeMedia{}
	var errOut bytes.Buffer
	deps := Dependencies{Service: svc, Media: media, Stdout: io.Discard, Stderr: &errOut}
	if code := Run(context.Background(), []string{"play", "7100000000000101", "--autoplay"}, deps); code != ExitOK {
		t.Fatalf("exit %d (%s)", code, errOut.String())
	}
	if strings.Join(media.streamed, " ") != epURL(1)+" "+epURL(3) {
		t.Fatalf("streamed = %q", media.streamed)
	}
	if !strings.Contains(errOut.String(), "episode 2 is not available anonymously; skipping") {
		t.Fatalf("stderr = %q", errOut.String())
	}
}

func TestPlayAutoplayStopsAfterUnavailableRun(t *testing.T) {
	eps := threeEpisodes()
	fail := map[string]error{}
	for _, e := range eps {
		fail[e.ID] = sdk.ErrAnonymousLimited
	}
	svc := &seqService{playService: playService{episodes: eps}, fail: fail}
	deps := Dependencies{Service: svc, Media: &fakeMedia{}, Stdout: io.Discard, Stderr: io.Discard}
	if code := Run(context.Background(), []string{"play", "7100000000000101", "--autoplay"}, deps); code != ExitUnavailable {
		t.Fatalf("exit %d", code)
	}
}

func TestDownloadSavesRangeAndSkipsExisting(t *testing.T) {
	svc := &seqService{playService: playService{episodes: threeEpisodes()}}
	media := &fakeMedia{writeFiles: true}
	dir := t.TempDir()
	var out bytes.Buffer
	deps := Dependencies{Service: svc, Media: media, Stdout: &out, Stderr: io.Discard}
	if err := os.WriteFile(filepath.Join(dir, "第001集.mp4"), []byte("done"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := Run(context.Background(), []string{"download", "7100000000000101", "--episodes", "1-2", "--dir", dir, "--jobs", "2"}, deps); code != ExitOK {
		t.Fatalf("exit %d", code)
	}
	if len(media.saved) != 1 || media.saved[0] != epURL(2) {
		t.Fatalf("saved = %q", media.saved)
	}
	for _, name := range []string{"第001集.mp4", "第002集.mp4"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "第003集.mp4")); err == nil {
		t.Fatal("episode outside the range was saved")
	}
	if !strings.Contains(out.String(), "1\t"+filepath.Join(dir, "第001集.mp4")+"\texists") {
		t.Fatalf("stdout = %q", out.String())
	}
	if matches, _ := filepath.Glob(filepath.Join(dir, "*.part")); len(matches) != 0 {
		t.Fatalf("leftover parts %q", matches)
	}
}

func TestDownloadDefaultsToTitleDirAndReportsFailures(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	svc := &seqService{playService: playService{episodes: threeEpisodes()},
		fail: map[string]error{"7100000000000203": sdk.ErrAnonymousLimited}}
	deps := Dependencies{Service: svc, Media: &fakeMedia{writeFiles: true}, Stdout: io.Discard, Stderr: io.Discard}
	if code := Run(context.Background(), []string{"download", "7100000000000101"}, deps); code != ExitFailure {
		t.Fatalf("exit %d", code)
	}
	// The title 剧/名 is made safe for a directory name.
	files, _ := filepath.Glob(filepath.Join("剧_名", "*.mp4"))
	if len(files) != 2 {
		t.Fatalf("files = %q", files)
	}
}

func TestParseEpisodeRange(t *testing.T) {
	got, err := parseEpisodeRange("3, 1-2,2", 5)
	if err != nil || len(got) != 3 || got[0] != 1 || got[2] != 3 {
		t.Fatalf("got %v err %v", got, err)
	}
	if all, _ := parseEpisodeRange("", 4); len(all) != 4 || all[3] != 4 {
		t.Fatalf("all = %v", all)
	}
	for _, bad := range []string{"0", "2-1", "x", "1-9"} {
		if _, err := parseEpisodeRange(bad, 5); err == nil {
			t.Fatalf("%q accepted", bad)
		}
	}
}

func TestHistoryEntryKeepsPosition(t *testing.T) {
	store := history.NewStore(filepath.Join(t.TempDir(), "h.json"))
	if err := store.Record(context.Background(), history.Entry{SeriesID: "1", Episode: 2, Position: 12.5}); err != nil {
		t.Fatal(err)
	}
	if e, _, _ := store.Get(context.Background(), "1"); e.Position != 12.5 {
		t.Fatalf("entry = %+v", e)
	}
}

func TestPlayAndDownloadRefuseBooks(t *testing.T) {
	svc := &seqService{playService: playService{episodes: threeEpisodes()}, book: true}
	media := &fakeMedia{writeFiles: true}
	var errOut bytes.Buffer
	deps := Dependencies{Service: svc, Media: media, Stdout: io.Discard, Stderr: &errOut}
	for _, args := range [][]string{
		{"play", "7100000000000101"},
		{"download", "7100000000000101", "--dir", t.TempDir()},
	} {
		if code := Run(context.Background(), args, deps); code != ExitUsage {
			t.Fatalf("%v exit %d", args, code)
		}
	}
	if !strings.Contains(errOut.String(), "is a book or audiobook, not a video") {
		t.Fatalf("stderr = %q", errOut.String())
	}
	if len(svc.resolved) != 0 || len(media.streamed) != 0 || len(media.saved) != 0 {
		t.Fatalf("resolved %q streamed %q saved %q", svc.resolved, media.streamed, media.saved)
	}
}
