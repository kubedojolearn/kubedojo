package progress

import (
	"testing"
	"time"
)

func date(y int, m time.Month, d int) *time.Time {
	t := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	return &t
}

func TestCurrentStreak(t *testing.T) {
	// 23:30 UTC on 22 Oct is already 23 Oct in Colombo (UTC+5:30).
	now := time.Date(2026, 10, 22, 23, 30, 0, 0, time.UTC)
	cases := []struct {
		name   string
		stored int32
		last   *time.Time
		tz     string
		want   int32
	}{
		{"completed today", 5, date(2026, 10, 22), "UTC", 5},
		{"completed yesterday", 5, date(2026, 10, 21), "UTC", 5},
		{"lapsed", 5, date(2026, 10, 20), "UTC", 0},
		{"never completed", 0, nil, "UTC", 0},
		{"user's day already rolled over keeps yesterday", 3, date(2026, 10, 22), "Asia/Colombo", 3},
		{"user's day rolled over: two days ago lapses", 3, date(2026, 10, 21), "Asia/Colombo", 0},
		{"behind UTC: still the 22nd in Los Angeles", 2, date(2026, 10, 21), "America/Los_Angeles", 2},
		{"unknown zone falls back to UTC", 2, date(2026, 10, 21), "Not/AZone", 2},
	}
	for _, tc := range cases {
		if got := CurrentStreak(tc.stored, tc.last, tc.tz, now); got != tc.want {
			t.Errorf("%s: got %d want %d", tc.name, got, tc.want)
		}
	}
}
