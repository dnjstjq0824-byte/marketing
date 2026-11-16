//go:build !windows

package store

// diskFreeTotal is unknown off Windows: zero totals never alert.
func diskFreeTotal(_ string) (free, total uint64) { return 0, 0 }
