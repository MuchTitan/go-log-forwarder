//go:build unix

package inputtail

import (
	"fmt"
	"os"
	"syscall"
)

func getFileID(info os.FileInfo) (uint64, error) {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		return stat.Ino, nil
	}
	return 0, fmt.Errorf("failed to get file inode")
}
