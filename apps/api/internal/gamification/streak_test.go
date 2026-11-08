package gamification

import (
	"testing"
	"time"
)

func TestNextStreak(t *testing.T) {
	day := func(s string) *time.Time {
		d, err := time.Parse("2006-01-02", s)
		if err != nil {
			t.Fatal(err)
		}
		return &d
	}
	// 2026-11-08 23:30 in Colombo is 18:00 UTC on the same day.
	now := time.Date(2026, 11, 8, 18, 0, 0, 0, time.UTC)
	cases := []struct {
		current int32
		last    *time.Time
		tz      string
		want    int32
		today   string
	}{
		{0, nil, "UTC", 1, "2026-11-08"},
		{4, day("2026-11-08"), "UTC", 4, "2026-11-08"},
		{4, day("2026-11-07"), "UTC", 5, "2026-11-08"},
		{4, day("2026-11-06"), "UTC", 1, "2026-11-08"},
		{2, day("2026-11-08"), "Asia/Colombo", 2, "2026-11-08"},
		// In Auckland it is already 9 November, so a completion "yesterday" there was 8 November.
		{2, day("2026-11-08"), "Pacific/Auckland", 3, "2026-11-09"},
		{2, day("2026-11-07"), "Not/AZone", 3, "2026-11-08"},
	}
	for _, c := range cases {
		today, got := NextStreak(c.current, c.last, c.tz, now)
		if got != c.want || today.Format("2006-01-02") != c.today {
			t.Errorf("%+v: got %d on %s", c, got, today.Format("2006-01-02"))
		}
	}
}
