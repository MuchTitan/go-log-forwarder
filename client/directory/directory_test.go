package directory_test

import (
	"log-forwarder-client/directory"
	"log-forwarder-client/reader"
	"testing"

	"github.com/stretchr/testify/assert"
)

func testCheckIfFileIsRead(t *testing.T) {
	t.Run("Tests if a the checkIfFileisRead function works", func(t *testing.T) {
		testReader := directory.StartReading("./test.log")
		runningReaders := []*reader.Reader{}
		runningReaders = append(runningReaders, testReader)

		result1 := directory.CheckIfFileIsRead(runningReaders, "./test.log")
		assert.Equal(t, true, result1, "The result should be true")
		testReader.Stop()
		result2 := directory.CheckIfFileIsRead(runningReaders, "./test.log")
		assert.Equal(t, false, result2, "The result should be false ")
	})
}

func testCleanUpRunningReaders(t *testing.T) {
	t.Run("Test if the runningReaders cleanup function works", func(t *testing.T) {
		reader1 := directory.StartReading("./test.log")
		reader2 := directory.StartReading("./test1.log")
		runningReaders1 := []*reader.Reader{reader1}
		runningReaders2 := []*reader.Reader{reader2}

		result := directory.CleanUpRunningReaders(runningReaders1, runningReaders2)
		assert.Equal(t, []*reader.Reader{reader1, reader2}, result, "foo")
	})
}
