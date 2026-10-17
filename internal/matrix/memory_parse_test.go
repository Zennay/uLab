package matrix

import (
	"strings"
	"testing"
)

func TestParseProcMemoryValidSyntheticInput(t *testing.T) {
	text := []byte("MemTotal:     10240 kB\nMemFree:        128 kB\nMemAvailable:  3072 kB\n")
	info, err := parseProcMemory(text)
	if err != nil {
		t.Fatal(err)
	}
	if info.TotalBytes != 10240*1024 || info.AvailableBytes != 3072*1024 {
		t.Fatalf("unexpected memory conversion: %+v", info)
	}
}

func TestParseProcMemoryMalformedRejects(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"overflow_value", "MemTotal: 18446744073709551615 kB\nMemAvailable: 1 kB\n"},
		{"overflow_available", "MemTotal: 1024 kB\nMemAvailable: 18446744073709551615 kB\n"},
		{"unknown_unit", "MemTotal: 1024 kB\nMemAvailable: 10 zebibytes\n"},
		{"duplicate_available", "MemTotal: 1024 kB\nMemAvailable: 10 kB\nMemAvailable: 11 kB\n"},
		{"duplicate_total", "MemTotal: 1024 kB\nMemTotal: 1024 kB\nMemAvailable: 10 kB\n"},
		{"greater_than_total", "MemTotal: 100 kB\nMemAvailable: 200 kB\n"},
		{"missing_total", "MemAvailable: 12 kB\n"},
		{"missing_available", "MemTotal: 12 kB\n"},
		{"garbled_available", "MemTotal: 1024 kB\nMemAvailable: corrupt kB\n"},
		{"missing_value", "MemTotal: 1024 kB\nMemAvailable:\n"},
		{"extra_words", "MemTotal: 1024 kB\nMemAvailable: 12 kB misleading\n"},
		{"empty_available", "MemTotal: 12 kB\nMemAvailable: 0 kB\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := parseProcMemory([]byte(tc.raw)); err == nil {
				t.Fatalf("admitted malformed memory telemetry: %s", strings.ReplaceAll(tc.raw, "\n", "\\n"))
			}
		})
	}
}
