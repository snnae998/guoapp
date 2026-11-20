package cliapp

import (
	"context"
	"strings"

	"github.com/beilunyang/openhongguo/sdk"
	"github.com/beilunyang/openhongguo/sdk/model"
)

func cmdRank(ctx context.Context, args []string, d Dependencies) error {
	if err := requireService(d); err != nil {
		return err
	}
	f, err := parseFlags(args)
	if err != nil {
		return err
	}
	if len(f.rest) != 0 {
		return usage("rank takes no arguments")
	}
	var ranking *model.Ranking
	var err2 error
	if f.shorts {
		if f.offsetSet {
			return usage("rank --shorts pages with --cursor, not --offset")
		}
		page, err := d.Service.ShortRank(ctx, sdk.ShortRankQuery{Content: f.content, Board: f.board, Cursor: f.cursor})
		if err != nil {
			return err
		}
		return printValue(d.Stdout, page, f.json)
	}
	if f.content != "" || f.board != "" {
		return usage("--content and --board need --shorts")
	}
	if f.groups {
		groups, err := d.Service.RankGroups(ctx)
		if err != nil {
			return err
		}
		ranking := &model.Ranking{Lists: groups, Items: []model.RankItem{}}
		return printValue(d.Stdout, ranking, f.json)
	}
	if f.cell != "" || f.activity == "" {
		ranking, err2 = d.Service.RankRows(ctx, f.cell, f.offset)
	} else {
		ranking, err2 = d.Service.Ranking(ctx, f.activity, f.list)
	}
	if err2 != nil {
		return err2
	}
	return printValue(d.Stdout, ranking, f.json)
}

func cmdFilter(ctx context.Context, args []string, d Dependencies) error {
	if err := requireService(d); err != nil {
		return err
	}
	f, err := parseFlags(args)
	if err != nil {
		return err
	}
	if len(f.rest) != 0 {
		return usage("filter takes no arguments; select with --genre and --filter KEY=VALUE")
	}
	if f.panel {
		if len(f.filters) > 0 || f.cursor != "" {
			return usage("--panel lists the filter rows; it takes only --genre")
		}
		rows, err := d.Service.ShortFilterPanel(ctx, f.genre)
		if err != nil {
			return err
		}
		return printValue(d.Stdout, rows, f.json)
	}
	selections := map[string][]string{}
	if f.genre != "" {
		selections["genre"] = []string{f.genre}
	}
	for _, kv := range f.filters {
		key, value, ok := strings.Cut(kv, "=")
		if !ok || key == "" || value == "" {
			return usage("--filter takes KEY=VALUE, got %q", kv)
		}
		selections[key] = append(selections[key], value)
	}
	page, err := d.Service.ShortFilter(ctx, sdk.ShortFilterQuery{Selections: selections, Cursor: f.cursor})
	if err != nil {
		return err
	}
	return printValue(d.Stdout, page, f.json)
}
