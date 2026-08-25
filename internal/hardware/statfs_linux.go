//go:build !windows

package hardware

import "syscall"

func getDiskSpace(mount string) (total uint64, free uint64, ok bool) {
	var st syscall.Statfs_t
	if syscall.Statfs(mount, &st) != nil {
		return 0, 0, false
	}
	return st.Blocks * uint64(st.Bsize), st.Bavail * uint64(st.Bsize), true
}
