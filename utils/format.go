package utils

import "fmt"

const (
	// Bold begins bold terminal text.
	Bold = "\033[1m"
	// Red begins red terminal text.
	Red = "\033[0;31m"
	// Green begins green terminal text.
	Green = "\033[0;32m"
	// Yellow begins yellow terminal text.
	Yellow = "\033[0;33m"
	// Gray begins gray terminal text.
	Gray = "\033[0;90m"
	// Reset restores default terminal styling.
	Reset = "\033[0m"
)

// HumanBytes formats a byte count using binary units.
func HumanBytes(bytes int) string {
	switch {
	case bytes >= 1024*1024*1024:
		return fmt.Sprintf("%.1f GiB", float64(bytes)/(1024*1024*1024))
	case bytes >= 1024*1024:
		return fmt.Sprintf("%.1f MiB", float64(bytes)/(1024*1024))
	case bytes >= 1024:
		return fmt.Sprintf("%.1f KiB", float64(bytes)/1024)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
