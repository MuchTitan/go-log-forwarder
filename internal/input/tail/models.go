package inputtail

import (
	"time"
)

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

type fileEventType int

const (
	FILEEVENT_CREATE fileEventType = iota
	FILEEVENT_WRITE
	FILEEVENT_DELETE
)

type fileEvent struct {
	path      string
	inode     uint64
	eventType fileEventType
}
