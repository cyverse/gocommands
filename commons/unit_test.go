package commons

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnit(t *testing.T) {
	t.Run("test Size", testSize)
	t.Run("test Time", testTime)
}

func testSize(t *testing.T) {
	s1 := "256b"
	s1b, err := ParseSize(s1)
	assert.NoError(t, err)
	assert.Equal(t, int64(256), s1b)

	s2 := "256"
	s2b, err := ParseSize(s2)
	assert.NoError(t, err)
	assert.Equal(t, int64(256), s2b)

	s3 := "256k"
	s3b, err := ParseSize(s3)
	assert.NoError(t, err)
	assert.Equal(t, int64(256*1024), s3b)

	s4 := "256kb"
	s4b, err := ParseSize(s4)
	assert.NoError(t, err)
	assert.Equal(t, int64(256*1024), s4b)

	s5 := "256m"
	s5b, err := ParseSize(s5)
	assert.NoError(t, err)
	assert.Equal(t, int64(256*1024*1024), s5b)

	s6 := "256x"
	_, err = ParseSize(s6)
	assert.Error(t, err)
}

func testTime(t *testing.T) {
	s1 := "32s"
	s1b, err := ParseTime(s1)
	assert.NoError(t, err)
	assert.Equal(t, 32, s1b)

	s2 := "32"
	s2b, err := ParseTime(s2)
	assert.NoError(t, err)
	assert.Equal(t, 32, s2b)

	s3 := "32m"
	s3b, err := ParseTime(s3)
	assert.NoError(t, err)
	assert.Equal(t, 32*60, s3b)

	s4 := "32h"
	s4b, err := ParseTime(s4)
	assert.NoError(t, err)
	assert.Equal(t, 32*60*60, s4b)

	s5 := "32d"
	s5b, err := ParseTime(s5)
	assert.NoError(t, err)
	assert.Equal(t, 32*60*60*24, s5b)

	s6 := "32e"
	_, err = ParseTime(s6)
	assert.Error(t, err)
}
