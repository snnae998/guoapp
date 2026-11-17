package media

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// streamLauncher stubs ffmpeg and a player with shell scripts in one
// directory. Each script can write files next to itself for the test to read.
func streamLauncher(t *testing.T, player, ffmpegBody, playerBody string) (*Launcher, string) {
	t.Helper()
	dir := t.TempDir()
	scripts := map[string]string{"ffmpeg": ffmpegBody, player: playerBody}
	for name, body := range scripts {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return &Launcher{
		Out: io.Discard, Err: io.Discard,
		LookPath: func(name string) (string, error) {
			if _, ok := scripts[name]; ok {
				return filepath.Join(dir, name), nil
			}
			return "", errors.New("missing")
		},
	}, dir
}

const recordArgs = `printf '%s\n' "$@" > "$(dirname "$0")/$(basename "$0")-args.txt"` + "\n"

func readArgs(t *testing.T, dir, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, name+"-args.txt"))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// ffplayStatus prints ffplay -stats lines for the given positions.
func ffplayStatus(positions ...string) string {
	var b strings.Builder
	b.WriteString(`printf '    nan    :  0.000 fd=   0 aq=    0KB\r`)
	for _, p := range positions {
		b.WriteString("  " + p + ` A-V:  0.001 fd=   0\r`)
	}
	b.WriteString("' >&2\n")
	return b.String()
}

func TestPlayLetsFFplayDecryptAndSeek(t *testing.T) {
	l, dir := streamLauncher(t, "ffplay", "exit 1\n", recordArgs+ffplayStatus("0.03", "40.10"))
	key := strings.Repeat("0123456789abcdef", 2)
	req := Request{URL: "https://media.invalid/drm.mp4", Key: key, Player: "ffplay", DurationSeconds: 42}
	res, err := l.Play(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if res.Position < 40 || res.Position > 40.1 {
		t.Fatalf("position = %v", res.Position)
	}
	args := readArgs(t, dir, "ffplay")
	for _, want := range []string{"-stats", "-decryption_key\n" + key, "https://media.invalid/drm.mp4"} {
		if !strings.Contains(args, want) {
			t.Fatalf("missing %q in ffplay args:\n%s", want, args)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "ffmpeg-args.txt")); err == nil {
		t.Fatal("ffplay playback also ran ffmpeg")
	}
}

func TestPlayFFplayStartsMidEpisode(t *testing.T) {
	// After -ss 30 ffplay reports stream time from 30 on.
	l, dir := streamLauncher(t, "ffplay", "", recordArgs+ffplayStatus("30.02", "40.00"))
	req := Request{URL: "https://media.invalid/clear.mp4", Player: "ffplay", StartSeconds: 30, DurationSeconds: 42}
	res, err := l.Play(context.Background(), req)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if res.Position < 39.9 || res.Position > 40.1 {
		t.Fatalf("position = %v", res.Position)
	}
	if args := readArgs(t, dir, "ffplay"); !strings.Contains(args, "-ss\n30\n") {
		t.Fatalf("ffplay args:\n%s", args)
	}
}

func TestPlayReportsFFplayClosedBeforeEnd(t *testing.T) {
	l, _ := streamLauncher(t, "ffplay", "", ffplayStatus("0.03", "12.50"))
	req := Request{URL: "https://media.invalid/clear.mp4", Player: "ffplay", DurationSeconds: 42}
	res, err := l.Play(context.Background(), req)
	if !errors.Is(err, ErrStopped) || res.Position < 12 || res.Position > 12.5 {
		t.Fatalf("err = %v position = %v", err, res.Position)
	}
	// A preview ends early on purpose.
	req.LimitSeconds = 10
	if _, err := l.Play(context.Background(), req); err != nil {
		t.Fatalf("preview err = %v", err)
	}
	// ffplay exits 123 from its SIGTERM handler.
	killed, _ := streamLauncher(t, "ffplay", "", "exit 123\n")
	if _, err := killed.Play(context.Background(), req); !errors.Is(err, ErrStopped) {
		t.Fatalf("killed err = %v", err)
	}
}

func TestPlayReportsFFplayThatNeverPlayed(t *testing.T) {
	// ffplay exits 0 even when the URL cannot be opened.
	l, _ := streamLauncher(t, "ffplay", "", "echo 'https://media.invalid/x.mp4: Server returned 404' >&2\n")
	_, err := l.Play(context.Background(), Request{URL: "https://media.invalid/x.mp4", Player: "ffplay", DurationSeconds: 42})
	if err == nil || errors.Is(err, ErrStopped) {
		t.Fatalf("err = %v", err)
	}
}

func TestPlayStripsFFplayTerminalEscapes(t *testing.T) {
	status := `printf '\033[2K   0.03 A-V:  0.001 fd=   0\r\033[2K  41.00 A-V:  0.001 fd=   0\r\033[2K\n' >&2` + "\n"
	l, _ := streamLauncher(t, "ffplay", "", status+"echo\n")
	var errOut, out bytes.Buffer
	l.Err, l.Out = &errOut, &out
	if _, err := l.Play(context.Background(), Request{URL: "https://media.invalid/clear.mp4", Player: "ffplay", DurationSeconds: 42}); err != nil {
		t.Fatal(err)
	}
	if errOut.Len() != 0 || out.Len() != 0 {
		t.Fatalf("player output leaked: %q %q", errOut.String(), out.String())
	}
}

// mpvEnd prints what the ohg Lua script prints when a file ends.
func mpvEnd(reason, pos string) string {
	return "echo '[ohg_x] ohg-end " + reason + " " + pos + "'\n"
}

func TestPlayMPVDecryptsAndReportsEnd(t *testing.T) {
	status := "echo 'AV: 00:00:03 / 00:03:28 (2%) A-V:  0.000 Cache: 3.2s/459KB'\n"
	l, dir := streamLauncher(t, "mpv", "", recordArgs+status+mpvEnd("eof", "207.500"))
	var out bytes.Buffer
	l.Out = &out
	key := strings.Repeat("ab", 16)
	req := Request{URL: "https://media.invalid/v.mp4", Audio: "https://media.invalid/a.m4a", Key: key, Player: "mpv", StartSeconds: 12}
	res, err := l.Play(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if res.Position != 207.5 {
		t.Fatalf("position = %v", res.Position)
	}
	if out.Len() != 0 {
		t.Fatalf("status line leaked: %q", out.String())
	}
	args := readArgs(t, dir, "mpv")
	for _, want := range []string{"--demuxer-lavf-o=decryption_key=" + key, "--audio-file=https://media.invalid/a.m4a", "--start=12", "--script="} {
		if !strings.Contains(args, want) {
			t.Fatalf("missing %q in mpv args:\n%s", want, args)
		}
	}
}

func TestPlayMPVQuitAndError(t *testing.T) {
	req := Request{URL: "https://media.invalid/v.mp4", Player: "mpv"}
	quit, _ := streamLauncher(t, "mpv", "", mpvEnd("quit", "33.000"))
	res, err := quit.Play(context.Background(), req)
	if !errors.Is(err, ErrStopped) || res.Position != 33 {
		t.Fatalf("quit err = %v position = %v", err, res.Position)
	}
	failed, _ := streamLauncher(t, "mpv", "", mpvEnd("error", "-1"))
	if _, err := failed.Play(context.Background(), req); err == nil || errors.Is(err, ErrStopped) {
		t.Fatalf("error err = %v", err)
	}
	// mpv exits 4 when a signal ends it.
	killed, _ := streamLauncher(t, "mpv", "", mpvEnd("quit", "8.000")+"exit 4\n")
	if res, err := killed.Play(context.Background(), req); !errors.Is(err, ErrStopped) || res.Position != 8 {
		t.Fatalf("killed err = %v position = %v", err, res.Position)
	}
	// Without Lua, mpv's own exit line still tells a quit apart.
	noLua, _ := streamLauncher(t, "mpv", "", "echo 'Exiting... (Quit)' >&2\n")
	if _, err := noLua.Play(context.Background(), req); !errors.Is(err, ErrStopped) {
		t.Fatalf("no-lua err = %v", err)
	}
}

func TestPlayMPVJumpKeys(t *testing.T) {
	req := Request{URL: "https://media.invalid/v.mp4", Player: "mpv"}
	next, _ := streamLauncher(t, "mpv", "", mpvEnd("quit", "5.000")+"exit 10\n")
	if res, err := next.Play(context.Background(), req); err != nil || res.Jump != JumpNext {
		t.Fatalf("next = %+v err = %v", res, err)
	}
	prev, _ := streamLauncher(t, "mpv", "", "exit 11\n")
	if res, err := prev.Play(context.Background(), req); err != nil || res.Jump != JumpPrevious {
		t.Fatalf("previous = %+v err = %v", res, err)
	}
}

func TestPlayPipesDecryptedStreamIntoVLC(t *testing.T) {
	l, dir := streamLauncher(t, "vlc", recordArgs+"printf 'clear-ts'\n", `cat > "$(dirname "$0")/played"`+"\n")
	key := strings.Repeat("0123456789abcdef", 2)
	if _, err := l.Play(context.Background(), Request{URL: "https://media.invalid/drm.mp4", Key: key, Player: "vlc", StartSeconds: 9}); err != nil {
		t.Fatal(err)
	}
	played, _ := os.ReadFile(filepath.Join(dir, "played"))
	if string(played) != "clear-ts" {
		t.Fatalf("player read %q", played)
	}
	args := readArgs(t, dir, "ffmpeg")
	for _, want := range []string{"-ss\n9\n-decryption_key\n" + key, "mpegts\npipe:1"} {
		if !strings.Contains(args, want) {
			t.Fatalf("missing %q in ffmpeg args:\n%s", want, args)
		}
	}
}

func TestPlayPipeReportsPlayerClosedEarly(t *testing.T) {
	l, _ := streamLauncher(t, "vlc", "exec sleep 5\n", "exit 0\n")
	start := time.Now()
	_, err := l.Play(context.Background(), Request{URL: "https://media.invalid/drm.mp4", Key: strings.Repeat("aa", 16), Player: "vlc"})
	if !errors.Is(err, ErrStopped) {
		t.Fatalf("err = %v", err)
	}
	if time.Since(start) > 3*time.Second {
		t.Fatal("ffmpeg was not stopped with the player")
	}
}

func TestPlayPipeReportsFFmpegFailure(t *testing.T) {
	l, _ := streamLauncher(t, "vlc", "echo 'Invalid data found' >&2; exit 1\n", "cat > /dev/null; exit 2\n")
	_, err := l.Play(context.Background(), Request{URL: "https://media.invalid/drm.mp4", Key: strings.Repeat("aa", 16), Player: "vlc"})
	if err == nil || !strings.Contains(err.Error(), "stream failed: Invalid data found") {
		t.Fatalf("err = %v", err)
	}
}

func TestPlayRejectsOpenerAndBadInput(t *testing.T) {
	l, _ := streamLauncher(t, "open", "", "exit 0\n")
	if _, err := l.Play(context.Background(), Request{URL: "https://media.invalid/a.mp4", Key: strings.Repeat("aa", 16), Player: "open"}); !errors.Is(err, ErrNeedsFile) {
		t.Fatalf("open err = %v", err)
	}
	if _, err := l.Play(context.Background(), Request{URL: "https://media.invalid/a.mp4", Player: "open"}); !errors.Is(err, ErrStopped) {
		t.Fatalf("open clear err = %v", err)
	}
	if _, err := l.Play(context.Background(), Request{URL: "file:///etc/passwd", Player: "open"}); err == nil {
		t.Fatal("file URL was accepted")
	}
	if _, err := l.Play(context.Background(), Request{URL: "https://media.invalid/a.mp4", Key: "zz", Player: "open"}); err == nil {
		t.Fatal("bad key was accepted")
	}
}
