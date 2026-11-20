//go:build live

package contract

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/beilunyang/openhongguo/adapters/filestore"
	"github.com/beilunyang/openhongguo/adapters/httptransport"
)

// TestLiveContract replays the CLI's calls against the live service:
//
//	go test -tags live -run TestLiveContract -v ./internal/contract
//
// OHG_CONFIG_DIR reuses an anonymous profile (default: a fresh one is
// registered), OHG_CONTRACT_QUERY picks the series to search for,
// OHG_CONTRACT_MOVIE adds a segmented title by series ID, and
// OHG_CONTRACT_OUT receives the live response shapes for a diff.
// OHG_API_ORIGIN and OHG_DISTRIBUTION_ORIGIN point the client at other hosts,
// as they do for the CLI.
func TestLiveContract(t *testing.T) {
	dir := os.Getenv("OHG_CONFIG_DIR")
	if dir == "" {
		dir = t.TempDir()
	}
	transport := httptransport.New()
	query := os.Getenv("OHG_CONTRACT_QUERY")
	if query == "" {
		query = "全球死考"
	}
	ffmpeg, _ := exec.LookPath("ffmpeg")
	if ffmpeg == "" {
		t.Log("ffmpeg not found; the derived key is not tried on a frame")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	checks := Run(ctx, Config{
		Transport:     transport,
		Profiles:      filestore.New(filepath.Join(dir, "anonymous.json")),
		FixtureDir:    filepath.Join("..", "..", "testdata", "protocol"),
		Query:         query,
		MovieSeriesID: os.Getenv("OHG_CONTRACT_MOVIE"),
		FFmpeg:        ffmpeg,

		APIOrigin:          os.Getenv("OHG_API_ORIGIN"),
		DistributionOrigin: os.Getenv("OHG_DISTRIBUTION_ORIGIN"),
	})
	var report strings.Builder
	for _, c := range checks {
		line := fmt.Sprintf("%-16s ok", c.Name)
		if !c.OK() {
			line = fmt.Sprintf("%-16s FAIL", c.Name)
		}
		if c.Fixture != "" {
			line += "  (" + c.Fixture + ")"
		}
		fmt.Fprintln(&report, line)
		if c.Err != nil {
			fmt.Fprintf(&report, "    error: %v\n", c.Err)
		}
		for _, d := range c.Drifts {
			fmt.Fprintf(&report, "    drift: %s\n", d)
		}
		if !c.OK() {
			t.Fail()
		}
	}
	t.Log("\n" + report.String())
	if out := os.Getenv("OHG_CONTRACT_OUT"); out != "" {
		if err := writeShapes(out, checks, report.String()); err != nil {
			t.Error(err)
		}
	}
}

// writeShapes saves the report and one shape file per check. Shapes hold
// paths and types only, so they are safe to keep as a CI artifact.
func writeShapes(dir string, checks []Check, report string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "report.txt"), []byte(report), 0o644); err != nil {
		return err
	}
	for _, c := range checks {
		if len(c.Live) == 0 {
			continue
		}
		var b strings.Builder
		for _, f := range c.Live {
			fmt.Fprintf(&b, "%s\t%s\n", f.Path, f.Type)
		}
		name := strings.ReplaceAll(c.Name, " ", "-") + ".shape.txt"
		if err := os.WriteFile(filepath.Join(dir, name), []byte(b.String()), 0o644); err != nil {
			return err
		}
	}
	return nil
}
