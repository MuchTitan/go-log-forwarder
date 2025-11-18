//go:build windows

package inputtail

import (
	"fmt"
	"os"
	"syscall"
)

func getFileID(info os.FileInfo) (uint64, error) {
	// On Windows, use creation time as a unique file identifier
	// This serves as a pseudo-inode since Windows doesn't have true inodes
	if stat, ok := info.Sys().(*syscall.Win32FileAttributeData); ok {
		return uint64(stat.CreationTime.Nanoseconds()), nil
	}
	return 0, fmt.Errorf("failed to get file identifier")
}
