package models

import (
	"context"
)

type LogLine struct {
  Data string `json:"data"`
  LineNum int `json:"lineNum"`
  TransmitionStatus bool `json:"transmitionStatus"`
}

type FileState struct {
  Path string `json:"path"`
  LastSendLine int `json:"lastSendLine"`
  State context.Context
  Cancel context.CancelFunc 
  LogLines []LogLine `json:"logLines"`
}

func CreateFile(path string) *FileState{
  return &FileState{
    Path: path,
    LastSendLine: 0,
    LogLines: []LogLine{},
  }
}

func (f *FileState)SetContext(parent context.Context) {
  ctx, cancel := context.WithCancel(parent)
  f.State = ctx
  f.Cancel = cancel
}
