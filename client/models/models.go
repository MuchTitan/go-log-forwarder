package models

import (
  "context"
)

type LogLine struct {
  Data string
  LineNum int
  TransmitionStatus bool
}

type LogFile struct {
  Path string
  LogLines *[]LogLine
}

type FileState struct {
  Path string
  LastSendLine int
  State context.Context
  Cancel context.CancelFunc 
}
