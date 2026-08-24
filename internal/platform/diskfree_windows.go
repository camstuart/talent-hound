//go:build windows

package platform

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// FreeDiskBytes reports the space available to the current user on the volume
// holding path.
func FreeDiskBytes(path string) (int64, error) {
	var free, total, totalFree uint64
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, fmt.Errorf("checking free space at %s: %w", path, err)
	}
	if err := windows.GetDiskFreeSpaceEx(p, &free, &total, &totalFree); err != nil {
		return 0, fmt.Errorf("checking free space at %s: %w", path, err)
	}
	return int64(free), nil // #nosec G115 -- a volume's free bytes cannot exceed int64.
}
