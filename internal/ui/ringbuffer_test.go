package ui

import (
	"strings"
	"testing"
)

func TestRingBufferBasic(t *testing.T) {
	rb := NewRingBuffer(100)
	rb.Write([]byte("hello"))
	rb.Write([]byte(" world"))
	if rb.String() != "hello world" {
		t.Errorf("got %q, want %q", rb.String(), "hello world")
	}
}

func TestRingBufferOverflow(t *testing.T) {
	rb := NewRingBuffer(10)
	rb.Write([]byte("hello world")) // 11 bytes, exceeds 10
	s := rb.String()
	if len(s) > 10 {
		t.Errorf("buffer should be capped at 10 bytes, got %d", len(s))
	}
	// The tail of the data should be preserved
	if !strings.HasSuffix(s, "orld") {
		t.Errorf("expected tail of data to be preserved, got %q", s)
	}
}

func TestRingBufferReset(t *testing.T) {
	rb := NewRingBuffer(100)
	rb.Write([]byte("data"))
	rb.Reset()
	if rb.String() != "" {
		t.Errorf("after Reset, expected empty string, got %q", rb.String())
	}
}

func TestRingBufferBytes(t *testing.T) {
	rb := NewRingBuffer(100)
	data := []byte{1, 2, 3, 4, 5}
	rb.Write(data)
	got := rb.Bytes()
	for i, b := range data {
		if got[i] != b {
			t.Errorf("byte[%d]: got %d, want %d", i, got[i], b)
		}
	}
}
