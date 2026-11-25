package media

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

// ErrNeedsFile means the resolved player cannot play a protected stream
// itself or read one on stdin (open and xdg-open hand a path to another
// application), so the caller has to decrypt to a file first.
var ErrNeedsFile = errors.New("player cannot play a protected stream directly; use mpv, ffplay, or vlc")

// ErrStopped means the player was closed before the episode reached its end.
var ErrStopped = errors.New("playback stopped")

// Request is one playback: a stream URL, an optional separate audio track,
// the content key of a protected stream, and the player to use.
type Request struct {
	URL    string
	Audio  string
	Key    string
	Player string
	// StartSeconds starts playback that far into the episode.
	StartSeconds int
	// LimitSeconds plays only the leading seconds when positive.
	LimitSeconds int
	// DurationSeconds is the episode length; it lets Play tell a finished
	// episode from a closed player.
	DurationSeconds int
}

// Jump values a viewer can ask for while an episode plays (mpv only).
const (
	JumpPrevious = -1
	JumpNext     = 1
)

// Result is what the player reported about one playback.
type Result struct {
	// Position is where playback stopped, in seconds from the start of the
	// episode; 0 when the player does not report it.
	Position float64
	// Jump is JumpNext or JumpPrevious when the viewer asked to switch
	// episodes; Play then returns a nil error.
	Jump int
}

// endTolerance is how close to the end the last reported position must be
// for an episode to count as finished.
const endTolerance = 5.0

// mpv exits with these codes from the key bindings in mpvScript.
const (
	mpvExitNext     = 10
	mpvExitPrevious = 11
)

// mpvScript reports how playback ended and binds > and < to switching
// episodes. print() lines reach the terminal as "[<script>] ohg-end ...".
const mpvScript = `local pos = nil
mp.observe_property("time-pos", "number", function(_, v) if v then pos = v end end)
mp.register_event("end-file", function(e)
  print(string.format("ohg-end %s %.3f", e.reason or "unknown", pos or -1))
end)
mp.add_forced_key_binding(">", "ohg-next", function() mp.commandv("quit", "10") end)
mp.add_forced_key_binding("<", "ohg-previous", function() mp.commandv("quit", "11") end)
`

// Play plays req without writing a file and returns once the player exits.
//
// A clear stream goes to the player as a URL. ffplay and mpv decrypt a
// protected stream themselves with the content key, so seeking keeps working.
// Otherwise ffmpeg decrypts into an MPEG-TS pipe the player reads on stdin.
// Play returns ErrStopped when the player was closed before the end, as far
// as the player reports it, and ErrNeedsFile for open and xdg-open with a
// protected or segmented stream.
func (l *Launcher) Play(ctx context.Context, req Request) (Result, error) {
	key := ""
	if strings.TrimSpace(req.Key) != "" {
		k, err := normalizeKey(req.Key)
		if err != nil {
			return Result{}, err
		}
		key = k
	}
	if err := validateMediaURL(req.URL); err != nil {
		return Result{}, err
	}
	if req.Audio != "" {
		if err := validateMediaURL(req.Audio); err != nil {
			return Result{}, err
		}
	}
	name, playerPath, err := l.resolve(req.Player)
	if err != nil {
		return Result{}, err
	}
	switch {
	case name == "ffplay" && req.Audio == "":
		args := []string{"-autoexit", "-loglevel", "warning", "-stats"}
		if req.StartSeconds > 0 {
			args = append(args, "-ss", strconv.Itoa(req.StartSeconds))
		}
		if key != "" {
			args = append(args, "-decryption_key", key)
		}
		if req.LimitSeconds > 0 {
			args = append(args, "-t", strconv.Itoa(req.LimitSeconds))
		}
		return l.runPlayer(ctx, name, playerPath, append(args, req.URL), req)
	case name == "mpv":
		script, err := writeTemp("ohg-*.lua", mpvScript)
		if err != nil {
			return Result{}, err
		}
		defer os.Remove(script)
		args := []string{"--no-resume-playback", "--script=" + script}
		if key != "" {
			args = append(args, "--demuxer-lavf-o=decryption_key="+key)
		}
		if req.Audio != "" {
			args = append(args, "--audio-file="+req.Audio)
		}
		if req.StartSeconds > 0 {
			args = append(args, "--start="+strconv.Itoa(req.StartSeconds))
		}
		if req.LimitSeconds > 0 {
			args = append(args, "--length="+strconv.Itoa(req.LimitSeconds))
		}
		return l.runPlayer(ctx, name, playerPath, append(args, req.URL), req)
	case key == "" && req.Audio == "":
		return l.runPlayer(ctx, name, playerPath, playerArgs(name, req.URL), req)
	}
	stdinArg, ok := pipeInput(name)
	if !ok {
		return Result{}, ErrNeedsFile
	}
	return Result{}, l.playPipe(ctx, name, playerPath, stdinArg, key, req)
}

func writeTemp(pattern, body string) (string, error) {
	f, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", err
	}
	if _, err := f.WriteString(body); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", err
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

// runPlayer runs the player on its own and reads its output to learn how
// playback ended.
func (l *Launcher) runPlayer(ctx context.Context, name, playerPath string, args []string, req Request) (Result, error) {
	watch := &endWatcher{player: name, out: orDiscard(l.Err)}
	cmd := exec.CommandContext(ctx, playerPath, args...)
	cmd.Stdout = &endWatcher{player: name, out: orDiscard(l.Out), shared: watch}
	cmd.Stderr = watch
	runErr := cmd.Run()
	if ctx.Err() != nil {
		return Result{}, ctx.Err()
	}
	res := watch.result(req)
	if name == "mpv" {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			switch exitErr.ExitCode() {
			case mpvExitNext:
				return Result{Position: res.Position, Jump: JumpNext}, nil
			case mpvExitPrevious:
				return Result{Position: res.Position, Jump: JumpPrevious}, nil
			}
		}
	}
	if closedByUser(name, runErr) {
		return res, ErrStopped
	}
	if runErr != nil {
		return res, fmt.Errorf("player %s failed: %w", name, runErr)
	}
	switch watch.outcome(req) {
	case outcomeStopped:
		return res, ErrStopped
	case outcomeFailed:
		return res, fmt.Errorf("player %s could not play the stream", name)
	}
	return res, nil
}

// playPipe decrypts with ffmpeg into an MPEG-TS pipe the player reads on
// stdin. Such a stream cannot be seeked, so this is only for players and
// segmented titles that cannot be decrypted by the player itself.
func (l *Launcher) playPipe(ctx context.Context, name, playerPath, stdinArg, key string, req Request) error {
	lookup := l.LookPath
	if lookup == nil {
		lookup = exec.LookPath
	}
	ffmpegPath, err := lookup("ffmpeg")
	if err != nil {
		return errors.New("ffmpeg is not installed; it is required to stream media")
	}
	r, w, err := os.Pipe()
	if err != nil {
		return err
	}
	srcCtx, cancelSrc := context.WithCancel(ctx)
	defer cancelSrc()
	ff := exec.CommandContext(srcCtx, ffmpegPath, append(ffmpegInputArgs(req.URL, req.Audio, key, req.StartSeconds, req.LimitSeconds),
		"-c", "copy", "-f", "mpegts", "pipe:1")...)
	var ffErr bytes.Buffer
	ff.Stdout = w
	ff.Stderr = &ffErr
	play := exec.CommandContext(ctx, playerPath, playerArgs(name, stdinArg)...)
	play.Stdin = r
	play.Stdout = orDiscard(l.Out)
	play.Stderr = orDiscard(l.Err)
	if err := ff.Start(); err != nil {
		r.Close()
		w.Close()
		return err
	}
	w.Close()
	if err := play.Start(); err != nil {
		r.Close()
		cancelSrc()
		_ = ff.Wait()
		return fmt.Errorf("player %s failed: %w", name, err)
	}
	r.Close()

	ffDone := make(chan error, 1)
	go func() { ffDone <- ff.Wait() }()
	playErr := play.Wait()
	if ctx.Err() != nil {
		<-ffDone
		return ctx.Err()
	}
	var ffRunErr error
	select {
	case ffRunErr = <-ffDone:
	default:
		// The player quit while ffmpeg was still producing the stream.
		cancelSrc()
		<-ffDone
		return ErrStopped
	}
	if ffRunErr != nil {
		msg := strings.TrimSpace(ffErr.String())
		if playErr == nil && (msg == "" || strings.Contains(strings.ToLower(msg), "broken pipe")) {
			// ffmpeg failed writing because the player closed the pipe.
			return ErrStopped
		}
		if msg == "" {
			msg = ffRunErr.Error()
		}
		return fmt.Errorf("stream failed: %s", firstLine(msg))
	}
	if closedByUser(name, playErr) {
		return ErrStopped
	}
	if playErr != nil {
		return fmt.Errorf("player %s failed: %w", name, playErr)
	}
	return nil
}

type outcome int

const (
	outcomeFinished outcome = iota
	outcomeStopped
	outcomeFailed
)

// endWatcher reads a player's terminal output. ffplay -stats reports the
// playback position on lines ending in \r; the mpv script prints the end
// reason and position. Other non-blank lines go to out.
type endWatcher struct {
	player string
	out    io.Writer
	// shared, when set, is the watcher that holds the state; the player's
	// stdout feeds the watcher of its stderr.
	shared *endWatcher

	mu    sync.Mutex
	line  []byte
	first float64
	last  float64
	seen  bool
	// reason is mpv's end-file reason (eof, quit, error, ...).
	reason string
	// quit and eof come from mpv's own exit line, for an mpv without Lua.
	quit, eof bool
}

func (w *endWatcher) Write(p []byte) (int, error) {
	target := w
	if w.shared != nil {
		target = w.shared
	}
	target.mu.Lock()
	defer target.mu.Unlock()
	for _, c := range p {
		if c != '\r' && c != '\n' {
			w.line = append(w.line, c)
			continue
		}
		target.consume(string(w.line), w.out, c)
		w.line = w.line[:0]
	}
	return len(p), nil
}

func (w *endWatcher) consume(line string, out io.Writer, end byte) {
	line = ansiEscape.ReplaceAllString(line, "")
	switch w.player {
	case "ffplay":
		if strings.Contains(line, "fd=") {
			fields := strings.Fields(line)
			if len(fields) > 0 {
				if pos, err := strconv.ParseFloat(fields[0], 64); err == nil && !math.IsNaN(pos) {
					if !w.seen {
						w.first, w.seen = pos, true
					}
					w.last = pos
				}
			}
			return
		}
	case "mpv":
		if i := strings.Index(line, "ohg-end "); i >= 0 {
			fields := strings.Fields(line[i:])
			if len(fields) == 3 {
				w.reason = fields[1]
				if pos, err := strconv.ParseFloat(fields[2], 64); err == nil && pos >= 0 {
					w.last, w.seen = pos, true
				}
			}
			return
		}
		if t := strings.TrimSpace(line); strings.HasPrefix(t, "AV:") || strings.HasPrefix(t, "A:") || strings.HasPrefix(t, "V:") {
			// The status line; without a terminal mpv prints it line by line.
			return
		}
		if strings.Contains(line, "Exiting... (Quit)") {
			w.quit = true
		}
		if strings.Contains(line, "Exiting... (End of file)") {
			w.eof = true
		}
	}
	if strings.TrimSpace(line) != "" {
		fmt.Fprintf(out, "%s%c", line, end)
	}
}

// result is the position the player last reported, relative to the start
// of the episode.
func (w *endWatcher) result(req Request) Result {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.seen {
		return Result{}
	}
	switch w.player {
	case "ffplay":
		// ffplay reports stream time; the first report is where playback
		// began, which is req.StartSeconds into the episode.
		return Result{Position: w.last - w.first + float64(req.StartSeconds)}
	case "mpv":
		return Result{Position: w.last}
	}
	return Result{}
}

// outcome reports how the player's playback of req ended. A player that
// reports nothing is taken at its word that the episode ended.
func (w *endWatcher) outcome(req Request) outcome {
	w.mu.Lock()
	seen, last, first, reason, quit, eof := w.seen, w.last, w.first, w.reason, w.quit, w.eof
	w.mu.Unlock()
	switch w.player {
	case "mpv":
		switch reason {
		case "eof":
			return outcomeFinished
		case "quit", "stop":
			return outcomeStopped
		case "error":
			return outcomeFailed
		}
		if quit && !eof {
			return outcomeStopped
		}
		return outcomeFinished
	case "ffplay":
		if !seen {
			// ffplay exits 0 even when it cannot open the stream.
			return outcomeFailed
		}
		if req.DurationSeconds <= 0 || req.LimitSeconds > 0 {
			return outcomeFinished
		}
		played := last - first + float64(req.StartSeconds)
		if played >= float64(req.DurationSeconds)-endTolerance {
			return outcomeFinished
		}
		return outcomeStopped
	case "open", "xdg-open":
		// The opener returns as soon as it hands the URL over, so nothing
		// is known about the end and autoplay must not race ahead.
		return outcomeStopped
	}
	return outcomeFinished
}

// closedByUser reports whether the player was killed by a signal or exited
// with the code its own signal handler uses: 123 for ffplay, 4 for mpv.
func closedByUser(name string, err error) bool {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return false
	}
	if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() {
		return true
	}
	switch name {
	case "ffplay":
		return exitErr.ExitCode() == 123
	case "mpv":
		return exitErr.ExitCode() == 4
	}
	return false
}

// ansiEscape matches the terminal control sequences ffplay mixes into its
// status line.
var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

// pipeInput is the input argument that makes the player read stdin.
func pipeInput(name string) (string, bool) {
	switch name {
	case "mpv", "vlc":
		return "-", true
	case "ffplay":
		return "pipe:0", true
	}
	return "", false
}
