package server

import (
	"context"

	"github.com/Marcowich0/hoardboard/internal/category"
	"github.com/Marcowich0/hoardboard/internal/history"
)

// trendJSON is the Overview's stacked category series (Trend in
// web/src/api/types.ts): each day's Used space split as the donut splits
// today's, from the first Scan to today.
type trendJSON struct {
	Days []string `json:"days"`
	// Categories is every category in Rank order.
	Categories    []trendSeriesJSON `json:"categories"`
	Uncategorized []int64           `json:"uncategorized"`
	Unaccounted   []int64           `json:"unaccounted"`
}

type trendSeriesJSON struct {
	CategoryID category.ID `json:"categoryId"`
	Bytes      []int64     `json:"bytes"`
}

// trend splits each day of History by Rank (ranked, highest first) under
// the Assignments, so a byte in several categories is stacked once, in the
// highest-Ranked of them. Today's Used space is liveUsed, the donut's, so
// the last day's stack is the donut.
func (s *Server) trend(ctx context.Context, r ranking, liveUsed int64) (trendJSON, error) {
	p, err := category.MeasurePast(ctx, s.history, r.assigned, s.today())
	if err != nil {
		return trendJSON{}, err
	}
	used, err := s.usedDaily(ctx, p.From, len(p.Sizes))
	if err != nil {
		return trendJSON{}, err
	}
	n := len(p.Sizes)
	out := trendJSON{
		Days:          make([]string, n),
		Categories:    make([]trendSeriesJSON, len(r.ranked)),
		Uncategorized: make([]int64, n),
		Unaccounted:   make([]int64, n),
	}
	for i, id := range r.ranked {
		out.Categories[i] = trendSeriesJSON{CategoryID: id, Bytes: make([]int64, n)}
	}
	for d, sz := range p.Sizes {
		day := p.From + history.Day(d)
		u := used[d]
		switch {
		case day == s.today():
			u = liveUsed
		case u < 0:
			u = sz.Total() // nothing to shrink to, nothing Unaccounted
		}
		split := sz.Split(r.ranked, u)
		out.Days[d] = day.String()
		for i, part := range split.Categories {
			out.Categories[i].Bytes[d] = part.Bytes
		}
		out.Uncategorized[d] = split.Uncategorized
		out.Unaccounted[d] = split.Unaccounted
	}
	return out, nil
}

// usedDaily is the Disks' total Used space on each of the n days from
// from, as each day's last Scan recorded it; -1 on a day before Used space
// History began.
func (s *Server) usedDaily(ctx context.Context, from history.Day, n int) ([]int64, error) {
	out := make([]int64, n)
	if n == 0 {
		return out, nil
	}
	for i := range out {
		out[i] = -1
	}
	rows, err := s.history.TotalUsed(ctx, from+history.Day(n-1), n)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		if i := int(r.Day - from); i >= 0 && i < n {
			out[i] = r.Value
		}
	}
	return out, nil
}
