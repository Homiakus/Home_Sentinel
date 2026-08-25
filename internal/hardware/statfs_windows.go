//go:build windows

package hardware

func getDiskSpace(mount string) (total uint64, free uint64, ok bool) {
	return 0, 0, false
}
