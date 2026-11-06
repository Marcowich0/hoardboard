package server_test

import (
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/Marcowich0/hoardboard/internal/apitest"
)

// growth is the growth rate and forecast of GET /api/overview.
type growth struct {
	Today    string `json:"today"`
	Forecast *struct {
		BytesPerMonth int64  `json:"bytesPerMonth"`
		FullOn        string `json:"fullOn"`
	} `json:"forecast"`
	ForecastDays int `json:"forecastDays"`
}

const (
	gib = int64(1) << 30
	tib = int64(1) << 40
)

// scanDays scans once a day for len(usedOnDisk1) days, from the clock's day
// on, with disk1's Used space as given and disk2 and the cache constant:
// disk1 4 TiB, disk2 4 TiB, cache 1 TiB, 9 TiB in all. The clock ends on the
// last day.
func scanDays(t *testing.T, app *apitest.App, usedOnDisk1 []int64) {
	t.Helper()
	app.Mount.SetSpace("disk2", 4*tib, 3*tib)
	app.Mount.SetSpace("cache", 1*tib, 512*gib)
	for i, used := range usedOnDisk1 {
		if i > 0 {
			app.Clock.Advance(day)
		}
		app.Mount.SetSpace("disk1", 4*tib, used)
		app.Scan(t)
	}
}

// line is n days of Used space on disk1 growing by perDay from first.
func line(n int, first, perDay int64) []int64 {
	out := make([]int64, n)
	for i := range out {
		out[i] = first + int64(i)*perDay
	}
	return out
}

func overviewGrowth(t *testing.T, app *apitest.App) growth {
	t.Helper()
	var got growth
	app.GetJSON(t, "/api/overview", http.StatusOK, &got)
	return got
}

func TestAWeekOfGrowthGivesAGrowthRateAndTheDayTheArrayIsFull(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t) // 2026-03-02
	scanDays(t, app, line(7, 3*tib, 10*gib))

	got := overviewGrowth(t, app)
	// 10 GiB a day, an average month being 365.25/12 days. On 2026-03-08
	// 6.5 TiB + 60 GiB of 9 TiB is used: 2500 GiB left, 250 days.
	if got.Today != "2026-03-08" || got.ForecastDays != 7 || got.Forecast == nil ||
		got.Forecast.BytesPerMonth != 326820167680 || got.Forecast.FullOn != "2026-11-13" {
		t.Fatalf("overview = %+v (forecast %+v), want 7 days, 326820167680 bytes a month, full on 2026-11-13",
			got, got.Forecast)
	}
}

func TestSixDaysAreTooFewForAForecast(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	if got := overviewGrowth(t, app); got.Forecast != nil || got.ForecastDays != 0 {
		t.Fatalf("before any Scan: forecast %+v over %d days, want none over 0", got.Forecast, got.ForecastDays)
	}

	scanDays(t, app, line(6, 3*tib, 10*gib))

	if got := overviewGrowth(t, app); got.Forecast != nil || got.ForecastDays != 6 {
		t.Fatalf("after 6 days: forecast %+v over %d days, want none over 6", got.Forecast, got.ForecastDays)
	}
}

func TestAFlatOrShrinkingArrayHasNoForecast(t *testing.T) {
	t.Parallel()
	for name, perDay := range map[string]int64{"flat": 0, "shrinking": -10 * gib} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			app := apitest.Start(t)
			scanDays(t, app, line(10, 3*tib, perDay))

			if got := overviewGrowth(t, app); got.Forecast != nil || got.ForecastDays != 10 {
				t.Fatalf("forecast %+v over %d days, want none over 10", got.Forecast, got.ForecastDays)
			}
		})
	}
}

func TestDaysWithoutAScanCarryTheLastUsedSpaceForward(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t) // 2026-03-02
	scanDays(t, app, line(2, 3*tib, 10*gib))
	// No Scan for five days: they repeat the second day's Used space.
	app.Clock.Advance(6 * day)
	app.Mount.SetSpace("disk1", 4*tib, 3*tib+70*gib)
	app.Scan(t)

	got := overviewGrowth(t, app)
	// Daily totals 0, 10, 10, 10, 10, 10, 10, 70 GiB above 6.5 TiB on
	// 2026-03-02…09: the least-squares slope is 35/6 GiB a day, which is
	// 190645097813.3 bytes a month. On 2026-03-09 the line is at 110/3 GiB:
	// 2560 - 110/3 GiB left, 432.6 days, so full on day 433.
	if got.ForecastDays != 8 || got.Forecast == nil || got.Forecast.BytesPerMonth != 190645097813 ||
		got.Forecast.FullOn != "2027-05-16" {
		t.Fatalf("forecast %+v over %d days, want 190645097813 bytes a month, full on 2027-05-16, over 8 days",
			got.Forecast, got.ForecastDays)
	}
}

func TestOnlyTheLast90DaysAreFitted(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t) // 2026-03-02
	// Ten days with disk1 full, then 90 days growing from 3 TiB by 10 GiB a
	// day, up to 2026-06-09.
	used := slices.Repeat([]int64{4 * tib}, 10)
	scanDays(t, app, append(used, line(90, 3*tib, 10*gib)...))

	got := overviewGrowth(t, app)
	// Only the line counts: 10 GiB a day, 6.5 TiB + 890 GiB used of 9 TiB on
	// 2026-06-09, 1670 GiB left, 167 days.
	if got.Today != "2026-06-09" || got.ForecastDays != 90 || got.Forecast == nil ||
		got.Forecast.BytesPerMonth != 326820167680 || got.Forecast.FullOn != "2026-11-23" {
		t.Fatalf("overview = %+v (forecast %+v), want 90 days, 326820167680 bytes a month, full on 2026-11-23",
			got, got.Forecast)
	}
}

func TestTheLastScanOfADayGivesTheDaysUsedSpace(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t) // 2026-03-02
	app.Mount.SetSpace("disk2", 4*tib, 3*tib)
	for i, used := range line(7, 3*tib, 10*gib) {
		if i > 0 {
			app.Clock.Advance(16 * time.Hour) // the next morning
		}
		// A morning Scan with disk1 and the cache nearly full…
		app.Mount.SetSpace("disk1", 4*tib, 4*tib-gib)
		app.Mount.SetSpace("cache", 1*tib, 1*tib-gib)
		app.Scan(t)
		// …and an evening one with the cache back where it was.
		app.Clock.Advance(8 * time.Hour)
		app.Mount.SetSpace("disk1", 4*tib, used)
		app.Mount.SetSpace("cache", 1*tib, 512*gib)
		app.Scan(t)
	}
	app.Restart(t)

	got := overviewGrowth(t, app)
	// As with one Scan a day: 10 GiB a day, full in 250 days.
	if got.ForecastDays != 7 || got.Forecast == nil ||
		got.Forecast.BytesPerMonth != 326820167680 || got.Forecast.FullOn != "2026-11-13" {
		t.Fatalf("forecast %+v over %d days, want 326820167680 bytes a month, full on 2026-11-13, over 7 days",
			got.Forecast, got.ForecastDays)
	}
}

func TestADiskTakenOutOfTheArrayNoLongerCounts(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	// disk2 holds 1 TiB for six days, then Unraid no longer has it mounted,
	// while disk1 grows by 10 GiB a day.
	app.Mount.SetSpace("cache", 1*tib, 512*gib)
	app.Mount.SetSpace("disk2", 4*tib, 1*tib)
	for i, used := range line(7, 3*tib, 10*gib) {
		if i > 0 {
			app.Clock.Advance(day)
		}
		if i == 6 {
			app.Status.SetDisk("disk2", "fsStatus", "Unmounted")
		}
		app.Mount.SetSpace("disk1", 4*tib, used)
		app.Scan(t)
	}

	// Used space fell by 1 TiB on the last day: no forecast. Had disk2's
	// last 1 TiB been carried forward, the array would grow.
	if got := overviewGrowth(t, app); got.Forecast != nil || got.ForecastDays != 7 {
		t.Fatalf("forecast %+v over %d days, want none over 7", got.Forecast, got.ForecastDays)
	}
}
