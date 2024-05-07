package models

type LogLine struct {
  Data string
  LineNum int
  TransmitionStatus bool
}
type LogFile struct {
  Path string
  LogLines *[]LogLine
}
