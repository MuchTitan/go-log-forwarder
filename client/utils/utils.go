package utils

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"syscall"
)

type MultiWriter struct {
	writers []io.Writer
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

func NewMultiWriter(writers ...io.Writer) *MultiWriter {
	return &MultiWriter{writers: writers}
}

func CountLines(filePath string) (int, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lines := 0
	for scanner.Scan() {
		lines++
	}

	if err := scanner.Err(); err != nil {
		return 0, err
	}

	return lines, nil
}

func RemoveIndexFromSlice[T any](slice []T, index int) []T {
	if index < 0 || index >= len(slice) {
		// Return the original slice if index is out of bounds
		return slice
	}

	return append(slice[:index], slice[index+1:]...)
}

func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
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

func CreateChecksumForFirstThreeLines(filepath string) ([]byte, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("error opening file: %w", err)
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	hash := sha256.New()

	for i := 0; i < 3; i++ {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				// If we hit EOF before reading 3 lines, it's not an error
				// We'll just hash what we've read so far
				break
			}
			return nil, fmt.Errorf("error reading line: %w", err)
		}
		hash.Write(line)
	}

	return hash.Sum(nil), nil
}
