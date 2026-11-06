package server

import (
	"context"
	"math/big"

	"github.com/Marcowich0/hoardboard/internal/history"
)

// The growth rate and the full-disk forecast: a least-squares line through
// the array's total Used space, one point per day over the last
// forecastWindow days (history.Store.TotalUsed).
const (
	forecastWindow    = 90
	forecastMinPoints = 7
)

// daysPerMonth is an average month, 365.25/12 days, as a fraction.
var daysPerMonth = big.NewRat(36525, 1200)

// forecastJSON is Forecast in web/src/api/types.ts.
type forecastJSON struct {
	// BytesPerMonth is the fitted line's growth.
	BytesPerMonth int64 `json:"bytesPerMonth"`
	// FullOn is the day the fitted line reaches the array's size,
	// "YYYY-MM-DD".
	FullOn string `json:"fullOn"`
}

// forecast fits the last forecastWindow days of total Used space up to
// today, and says how many daily points the fit had. The forecast is nil
// with fewer than forecastMinPoints points, or when the line is flat or
// falling. size is the array's size now.
func (s *Server) forecast(ctx context.Context, today history.Day, size int64) (*forecastJSON, int, error) {
	points, err := s.history.TotalUsed(ctx, today, forecastWindow)
	if err != nil {
		return nil, 0, err
	}
	return fitForecast(points, size), len(points), nil
}

// fitForecast is the least-squares line through points (one per day, the
// last one today) and the day it reaches size, in exact arithmetic: the
// values are around 2^44 bytes, and a float would lose whole bytes per day
// and could tip the day the array is full.
func fitForecast(points []history.Row[int64], size int64) *forecastJSON {
	n := len(points)
	if n < forecastMinPoints {
		return nil
	}
	// With x the day's index and X = 2x - (n-1), which sums to zero, the
	// slope is Σ X·y / Σ X² · 2.
	var sxy, sxx, sy big.Int
	for i, p := range points {
		x := big.NewInt(int64(2*i - (n - 1)))
		y := big.NewInt(p.Value)
		sxy.Add(&sxy, new(big.Int).Mul(x, y))
		sxx.Add(&sxx, new(big.Int).Mul(x, x))
		sy.Add(&sy, y)
	}
	if sxy.Sign() <= 0 {
		return nil
	}
	perDay := new(big.Rat).SetFrac(new(big.Int).Lsh(&sxy, 1), &sxx)
	// The line's value today, at x = n-1: the mean plus (n-1)/2 days.
	usedToday := new(big.Rat).SetFrac(&sy, big.NewInt(int64(n)))
	usedToday.Add(usedToday, new(big.Rat).Mul(perDay, big.NewRat(int64(n-1), 2)))

	left := new(big.Rat).Sub(new(big.Rat).SetInt64(size), usedToday)
	days := int64(0)
	if left.Sign() > 0 {
		days = ceil(new(big.Rat).Quo(left, perDay))
	}
	return &forecastJSON{
		BytesPerMonth: round(new(big.Rat).Mul(perDay, daysPerMonth)),
		FullOn:        (points[n-1].Day + history.Day(days)).String(),
	}
}

// ceil is the smallest integer not below r.
func ceil(r *big.Rat) int64 {
	q, m := new(big.Int).DivMod(r.Num(), r.Denom(), new(big.Int))
	if m.Sign() != 0 {
		q.Add(q, big.NewInt(1))
	}
	return q.Int64()
}

// round is r rounded to the nearest integer, halves up.
func round(r *big.Rat) int64 {
	half := new(big.Rat).Add(r, big.NewRat(1, 2))
	q := new(big.Int).Div(half.Num(), half.Denom()) // floor: Denom > 0
	return q.Int64()
}
