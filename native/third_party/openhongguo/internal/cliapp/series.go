package cliapp

import (
	"context"
	"fmt"
	"strings"

	"github.com/beilunyang/openhongguo/sdk"
	"github.com/beilunyang/openhongguo/sdk/model"
)

func cmdInfo(ctx context.Context, args []string, d Dependencies) error {
	if err := requireService(d); err != nil {
		return err
	}
	f, err := parseFlags(args)
	if err != nil {
		return err
	}
	if len(f.rest) == 0 {
		return usage("info needs a series ID")
	}
	ids := make([]string, 0, len(f.rest))
	for _, arg := range f.rest {
		id, err := validateID(arg)
		if err != nil {
			return err
		}
		ids = append(ids, id)
	}
	if len(ids) == 1 {
		series, err := d.Service.GetSeries(ctx, ids[0])
		if err != nil {
			return err
		}
		return printValue(d.Stdout, series, f.json)
	}
	// Several IDs are read in one batch; unknown ones are left out.
	details, err := d.Service.GetSeriesBatch(ctx, ids)
	if err != nil {
		return err
	}
	list := make([]*model.Series, 0, len(details))
	for _, dt := range details {
		list = append(list, dt.Series)
	}
	if f.json {
		return printJSON(d.Stdout, list)
	}
	for i, series := range list {
		if i > 0 {
			fmt.Fprintln(d.Stdout)
		}
		if err := printValue(d.Stdout, series, false); err != nil {
			return err
		}
	}
	if len(list) < len(ids) {
		fmt.Fprintf(d.Stderr, "ohg: %d of %d series not found\n", len(ids)-len(list), len(ids))
	}
	return nil
}

func cmdEpisodes(ctx context.Context, args []string, d Dependencies) error {
	if err := requireService(d); err != nil {
		return err
	}
	f, err := parseFlags(args)
	if err != nil {
		return err
	}
	if len(f.rest) != 1 {
		return usage("episodes needs one series ID")
	}
	id, err := validateID(f.rest[0])
	if err != nil {
		return err
	}
	page, err := d.Service.ListEpisodes(ctx, id, sdk.EpisodeOptions{Cursor: f.cursor})
	if err != nil {
		return err
	}
	return printValue(d.Stdout, page, f.json)
}

func findEpisode(items []model.Episode, number int) (model.Episode, bool) {
	for _, e := range items {
		if e.Number == number {
			return e, true
		}
	}
	return model.Episode{}, false
}

// drmError names the protection scheme so the caller can tell a protected
// stream from a missing one. It never includes keys or per-segment data.
func drmError(p *model.Playback) error {
	scheme := ""
	for _, s := range p.Streams {
		if s.Encryption != "" {
			scheme = s.Encryption
			break
		}
	}
	if scheme == "" {
		return fmt.Errorf("%w: pass --key HEX, set OHG_CENC_KEY, or leave decryption enabled", sdk.ErrDRMProtected)
	}
	return fmt.Errorf("%w (%s): pass --key HEX, set OHG_CENC_KEY, or leave decryption enabled", sdk.ErrDRMProtected, scheme)
}

func cmdAlbum(ctx context.Context, args []string, d Dependencies) error {
	if err := requireService(d); err != nil {
		return err
	}
	f, err := parseFlags(args)
	if err != nil {
		return err
	}
	if len(f.rest) != 1 {
		return usage("album needs one album ID")
	}
	id, err := validateID(f.rest[0])
	if err != nil {
		return err
	}
	album, err := d.Service.GetAlbum(ctx, id, sdk.AlbumOptions{})
	if err != nil {
		return err
	}
	return printValue(d.Stdout, album, f.json)
}

func cmdRelated(ctx context.Context, args []string, d Dependencies) error {
	if err := requireService(d); err != nil {
		return err
	}
	f, err := parseFlags(args)
	if err != nil {
		return err
	}
	if len(f.rest) != 1 {
		return usage("related needs one series ID")
	}
	id, err := validateID(f.rest[0])
	if err != nil {
		return err
	}
	rec, err := d.Service.SeriesRecommend(ctx, id)
	if err != nil {
		return err
	}
	return printValue(d.Stdout, rec, f.json)
}

func cmdSeasons(ctx context.Context, args []string, d Dependencies) error {
	if err := requireService(d); err != nil {
		return err
	}
	f, err := parseFlags(args)
	if err != nil {
		return err
	}
	if len(f.rest) != 1 {
		return usage("seasons needs one series ID")
	}
	id, err := validateID(f.rest[0])
	if err != nil {
		return err
	}
	album, err := d.Service.Seasons(ctx, id)
	if err != nil && album == nil {
		return err
	}
	if err := printValue(d.Stdout, album, f.json); err != nil {
		return err
	}
	if err != nil {
		return err
	}
	return nil
}

// cmdPerson prints a person's page and the first page of their works; with
// --cursor it prints only the next page of works.
func cmdPerson(ctx context.Context, args []string, d Dependencies) error {
	if err := requireService(d); err != nil {
		return err
	}
	f, err := parseFlags(args)
	if err != nil {
		return err
	}
	if len(f.rest) != 1 {
		return usage("person needs one user ID")
	}
	userID := f.rest[0]
	var person *model.Person
	if f.cursor == "" {
		if person, err = d.Service.Person(ctx, userID); err != nil {
			return usageFromID(err)
		}
	}
	works, err := d.Service.PersonWorks(ctx, userID, f.cursor)
	if err != nil {
		return usageFromID(err)
	}
	if f.json {
		return printJSON(d.Stdout, struct {
			Person *model.Person   `json:"person,omitempty"`
			Works  *model.WorkPage `json:"works"`
		}{person, works})
	}
	if person != nil {
		fmt.Fprintf(d.Stdout, "%s\t%s\t%s\n", person.UserID, person.Name, strings.Join(append(append([]string{}, person.Tags...), person.Labels...), " · "))
		if person.Description != "" {
			fmt.Fprintln(d.Stdout, person.Description)
		}
		fmt.Fprintf(d.Stdout, "followers\t%d\tlikes\t%d\n", person.Followers, person.Likes)
		fmt.Fprintln(d.Stdout, "# 作品")
	}
	for _, w := range works.Items {
		fmt.Fprintf(d.Stdout, "%s\t%s\t%d\t%s\n", w.SeriesID, w.Title, w.EpisodeCount, strings.Join(w.Tags, " · "))
	}
	if works.Cursor != "" {
		fmt.Fprintf(d.Stdout, "cursor\t%s\n", works.Cursor)
	}
	return nil
}
