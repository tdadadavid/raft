package main

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
)

const RECURSION_FLAG uint16 = 1 << 8

func TestShouldEncodeHeaderToBytes(t *testing.T) {
	header := NewHeader(22, int64(RECURSION_FLAG), 1, 0, 0, 0)

	encodedHeader, err := header.ToBytes()
	assert.Nil(t, err)

	expected, err := hex.DecodeString("0016010000010000000000000")
	assert.NotNil(t, err)
	assert.Equal(t, encodedHeader, expected)
}