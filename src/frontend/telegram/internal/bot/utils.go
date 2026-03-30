package bot

import (
	"fmt"
	"strconv"
	"strings"
)

// --- Amount helpers ---

// parseAmount parses a ruble amount like "150", "99.50", "99,50" into kopecks.
func parseAmount(s string) (int64, error) {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ",", ".")
	parts := strings.SplitN(s, ".", 2)

	rubles, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || rubles < 0 {
		return 0, fmt.Errorf("invalid amount")
	}

	kopecks := int64(0)
	if len(parts) == 2 && parts[1] != "" {
		kStr := parts[1]
		switch len(kStr) {
		case 1:
			kStr += "0"
		default:
			kStr = kStr[:2]
		}
		kopecks, err = strconv.ParseInt(kStr, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid kopecks")
		}
	}
	return rubles*100 + kopecks, nil
}

// formatAmount converts kopecks to a human-readable ruble string.
func formatAmount(kopecks int64) string {
	r := kopecks / 100
	k := kopecks % 100
	if k == 0 {
		return fmt.Sprintf("%d", r)
	}
	return fmt.Sprintf("%d.%02d", r, k)
}
