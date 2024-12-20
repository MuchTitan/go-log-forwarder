package util

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"syscall"
)

type Event struct {
	ParsedData  map[string]interface{}
	Metadata    map[string]interface{}
	InputTag    string
	InputSource string
	RawData     []byte
	Time        int64
}

type MultiWriter struct {
	writers []io.Writer
}

func NewMultiWriter(writers ...io.Writer) *MultiWriter {
	return &MultiWriter{writers: writers}
}

func (mw *MultiWriter) AddWriter(w io.Writer) {
	mw.writers = append(mw.writers, w)
}

func (mw *MultiWriter) Write(p []byte) (n int, err error) {
	for _, w := range mw.writers {
		n, err = w.Write(p)
		if err != nil {
			return
		}
	}
	return len(p), nil
}

func TagMatch(inputTag, match string) bool {
	// Split the pattern by '*' and get the parts.
	if match == "" && inputTag != "" {
		return false
	}
	parts := strings.Split(match, "*")

	// Keep track of the current position in the input string.
	pos := 0

	for i, part := range parts {
		if part == "" {
			continue
		}

		// If it's the first part, the input string must start with this part.
		if i == 0 && !strings.HasPrefix(inputTag, part) {
			return false
		}

		// If it's the last part, the input string must end with this part.
		if i == len(parts)-1 && !strings.HasSuffix(inputTag, part) {
			return false
		}

		// Find the next occurrence of the part in the input string starting from `pos`.
		index := strings.Index(inputTag[pos:], part)
		if index == -1 {
			return false
		}

		// Move the position forward.
		pos += index + len(part)
	}

	return true
}

func GetInodeNumber(filepath string) (uint64, error) {
	fileInfo, err := os.Stat(filepath)
	if err != nil {
		return 0, err
	}
	stat, ok := fileInfo.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, fmt.Errorf("not a syscall.Stat_t")
	}
	return stat.Ino, nil
}

func CreateChecksumForFirstLine(filepath string) ([]byte, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("error opening file: %w", err)
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	hash := sha256.New()

	line, err := reader.ReadBytes('\n')
	if err != nil {
		if err != io.EOF {
			return nil, fmt.Errorf("error reading line: %w", err)
		}
	}
	hash.Write(line)

	return hash.Sum(nil), nil
}

func GetHostname() string {
	hostname, _ := os.Hostname()
	return hostname
}

func GetNameOfInterface(in interface{}) string {
	if in == nil {
		return ""
	}
	return reflect.TypeOf(in).Name()
}

// Merge the maps
func MergeMaps(m1, m2 map[string]interface{}) map[string]interface{} {
	for k, v := range m2 {
		m1[k] = v
	}
	return m1
}
