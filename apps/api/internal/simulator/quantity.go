package simulator

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// ParseCPU parses a CPU quantity ("500m", "1", "0.25", "2.5") into millicores.
func ParseCPU(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty CPU quantity")
	}
	if strings.HasSuffix(s, "m") {
		n, err := strconv.ParseInt(strings.TrimSuffix(s, "m"), 10, 64)
		if err != nil || n < 0 {
			return 0, fmt.Errorf("invalid CPU quantity %q", s)
		}
		return n, nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || f < 0 || math.IsInf(f, 0) || math.IsNaN(f) || f > 1e6 {
		return 0, fmt.Errorf("invalid CPU quantity %q", s)
	}
	return int64(math.Ceil(f * 1000)), nil
}

var memorySuffixes = []struct {
	suffix string
	factor float64
}{
	{"Ki", 1 << 10}, {"Mi", 1 << 20}, {"Gi", 1 << 30}, {"Ti", 1 << 40},
	{"k", 1e3}, {"M", 1e6}, {"G", 1e9}, {"T", 1e12},
}

// ParseMemory parses a memory quantity ("64Mi", "1Gi", "500M", "1048576") into bytes.
func ParseMemory(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty memory quantity")
	}
	factor := 1.0
	num := s
	for _, sfx := range memorySuffixes {
		if strings.HasSuffix(s, sfx.suffix) {
			factor, num = sfx.factor, strings.TrimSuffix(s, sfx.suffix)
			break
		}
	}
	f, err := strconv.ParseFloat(num, 64)
	if err != nil || f < 0 || math.IsInf(f, 0) || math.IsNaN(f) || f*factor > 1<<50 {
		return 0, fmt.Errorf("invalid memory quantity %q", s)
	}
	return int64(math.Ceil(f * factor)), nil
}

// ParseCount parses a plain integer quantity (node "pods").
func ParseCount(s string) (int64, error) {
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("invalid count %q", s)
	}
	return n, nil
}

// FormatCPU renders millicores the way kubectl does ("500m", "2").
func FormatCPU(milli int64) string {
	if milli%1000 == 0 {
		return strconv.FormatInt(milli/1000, 10)
	}
	return strconv.FormatInt(milli, 10) + "m"
}

// FormatMemory renders bytes with the largest exact binary suffix ("64Mi", "7Gi").
func FormatMemory(b int64) string {
	for _, sfx := range []struct {
		s string
		f int64
	}{{"Ti", 1 << 40}, {"Gi", 1 << 30}, {"Mi", 1 << 20}, {"Ki", 1 << 10}} {
		if b >= sfx.f && b%sfx.f == 0 {
			return strconv.FormatInt(b/sfx.f, 10) + sfx.s
		}
	}
	return strconv.FormatInt(b, 10)
}
