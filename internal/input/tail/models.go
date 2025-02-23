package inputtail

import "time"

type fileState struct {
	Path         string
	Offset       int64
	LastReadLine int
	InodeNumber  uint64
	CreatedAt    string
	UpdatedAt    string
}

type fileInfo struct {
	modTime time.Time
	size    int64
	inode   uint64
}
