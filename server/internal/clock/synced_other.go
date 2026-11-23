//go:build !linux

package clock

// NOTE: Outside Linux (development machines) the clock is taken as right.
func Synced() bool {
	return true
}
