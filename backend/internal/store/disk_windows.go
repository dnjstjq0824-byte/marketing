//go:build windows

package store

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// diskFreeTotal reports the free/total bytes of the volume holding path.
func diskFreeTotal(path string) (free, total uint64) {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	if _, err := os.Stat(abs); err != nil {
		abs = filepath.Dir(abs)
	}
	p, err := windows.UTF16PtrFromString(abs)
	if err != nil {
		return 0, 0
	}
	var avail, tot, freeBytes uint64
	if err := windows.GetDiskFreeSpaceEx(p, &avail, &tot, &freeBytes); err != nil {
		return 0, 0
	}
	return avail, tot
}
