package tail

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTailOnFile(t *testing.T) {
	t.Run("TestTailOnFile", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "test.log")
		if err != nil {
			t.Fatalf("Coundnt create a tmp File for testing: %v", err)
		}
		defer os.Remove(tmpFile.Name())

		tailer := NewFileTail(tmpFile.Name(), TailConfig{
			Offset: 0,
			ReOpen: false,
		})
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
		defer cancel()

		content := [][]byte{
			[]byte("Test1"),
			[]byte("Test2"),
		}

		for _, value := range content {
			tmpFile.Write(value)
			tmpFile.Write([]byte("\n"))
		}

		lineChan, err := tailer.Start(ctx)
		if err != nil {
			t.Fatalf("Coudnt open LineChan: %v", err)
		}

		for line := range lineChan {
			assert.Equal(t, string(content[line.LineNum-int64(1)]), line.LineData)
			fmt.Println(line.LineData)
		}
	},
	)
}
