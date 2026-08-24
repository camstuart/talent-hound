//go:build !windows

package platform

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// FreeDiskBytes reports the space available to the current user on the volume
// holding path.
func FreeDiskBytes(path string) (int64, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return 0, fmt.Errorf("checking free space at %s: %w", path, err)
	}
	return int64(st.Bavail) * int64(st.Bsize), nil // #nosec G115 -- available blocks cannot exceed int64 on any real volume.
}
