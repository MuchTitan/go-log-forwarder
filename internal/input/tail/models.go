package tail

type fileState struct {
	Path         string
	Offset       int64
	LastReadLine int
	InodeNumber  uint64
	CreatedAt    string
	UpdatedAt    string
}
