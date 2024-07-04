package reader_test

import (
	"log-forwarder-client/reader"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestReader(t *testing.T) {
	t.Run("Check if the Reading Works", func(t *testing.T) {
		reader := reader.New(reader.Config{Path: "test.log"})

		reader.Start()

		time.Sleep(time.Millisecond * 250)

		reader.Stop()

		assert.Equal(t, "foo", reader.Lines[0].Data)
		assert.Equal(t, "bar", reader.Lines[1].Data)
		assert.Equal(t, 1, reader.Lines[0].Num)
		assert.Equal(t, 2, reader.Lines[1].Num)
	})
}
