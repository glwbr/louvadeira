//go:build linux

package clock

import "golang.org/x/sys/unix"

/**
 * NOTE: Synced reports whether the kernel holds the clock as synchronized, the check behind timedatectl's
 * NTPSynchronized: a maximum error under 16 seconds, which NTP keeps low and a clock nobody set leaves high.
 */
func Synced() bool {
	var state unix.Timex
	if _, err := unix.Adjtimex(&state); err != nil {
		return false
	}
	return state.Maxerror < 16_000_000
}
