package config

import (
	"testing"
)

func TestByteSizeParsesUnits(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   string
		want int64
	}{
		{"1024", 1024},
		{"1KiB", 1024},
		{"1KB", 1000},
		{"512MiB", 512 << 20},
		{"1GiB", 1 << 30},
		{"2MB", 2_000_000},
	}
	for _, tt := range tests {
		var b ByteSize
		if err := b.UnmarshalText([]byte(tt.in)); err != nil {
			t.Errorf("UnmarshalText(%q): %v", tt.in, err)
			continue
		}
		if b.Int64() != tt.want {
			t.Errorf("UnmarshalText(%q) = %d, want %d", tt.in, b.Int64(), tt.want)
		}
	}
}

func TestByteSizeRejectsJunk(t *testing.T) {
	t.Parallel()
	for _, in := range []string{"", "MiB", "0", "-1", "12XB"} {
		var b ByteSize
		if err := b.UnmarshalText([]byte(in)); err == nil {
			t.Errorf("UnmarshalText(%q) = nil, want error", in)
		}
	}
}
