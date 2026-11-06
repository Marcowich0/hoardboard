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

// trend splits each day of History by Rank under the current Assignments
// and Rank, so a byte in several categories is stacked once, in the
// highest-Ranked of them. Today's Used space is liveUsed, the donut's, so
// the last day's stack is the donut.
func (s *Server) trend(ctx context.Context, liveUsed int64) (trendJSON, error) {
	p, err := s.measurePast(ctx)
	if err != nil {
		return trendJSON{}, err
	}
	used, err := s.usedDaily(ctx, p.from, len(p.sizes))
	if err != nil {
		return trendJSON{}, err
	}
	n := len(p.sizes)
	out := trendJSON{
		Days:          make([]string, n),
		Categories:    make([]trendSeriesJSON, len(p.ranked)),
		Uncategorized: make([]int64, n),
		Unaccounted:   make([]int64, n),
	}
	for i, id := range p.ranked {
		out.Categories[i] = trendSeriesJSON{CategoryID: id, Bytes: make([]int64, n)}
	}
	for d, sz := range p.sizes {
		day := p.from + history.Day(d)
		u := used[d]
		if day == s.today() {
			u = liveUsed
		}
		split := sz.Split(p.ranked, u)
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
// from, as each day's last Scan recorded it; 0 on a day before Used space
// History began (its Unaccounted is then 0).
func (s *Server) usedDaily(ctx context.Context, from history.Day, n int) ([]int64, error) {
	out := make([]int64, n)
	if n == 0 {
		return out, nil
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
