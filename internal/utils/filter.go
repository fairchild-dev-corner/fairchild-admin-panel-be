package utils

import cc "fairchild-admin-panel-be/internal/constants"

// TotalFor is how many rows a list holds for one tab - counts[tab], or every
// tab together for cc.FILTER_ALL.
func TotalFor(counts map[string]int, tab string) int {
	if tab != cc.FILTER_ALL {
		return counts[tab]
	}
	total := 0
	for _, n := range counts {
		total += n
	}
	return total
}

// IsTabFilter reports whether tab is one of tabs or cc.FILTER_ALL.
func IsTabFilter(tabs []string, tab string) bool {
	if tab == cc.FILTER_ALL {
		return true
	}
	for _, t := range tabs {
		if t == tab {
			return true
		}
	}
	return false
}
