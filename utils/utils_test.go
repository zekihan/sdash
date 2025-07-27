package utils

import (
	"testing"
)

func TestRandString(t *testing.T) {
	tests := []struct {
		name    string
		nByte   int
		wantErr bool
	}{
		{"ValidLength10", 10, false},
		{"ValidLength0", 0, false},
		{"NegativeLength", -1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := RandString(tt.nByte)
			if (err != nil) != tt.wantErr {
				t.Errorf("RandString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(got) == 0 && tt.nByte > 0 {
				t.Errorf("RandString() returned empty string for nByte %d", tt.nByte)
			}
			if !tt.wantErr && len(got) < tt.nByte*4/3 {
				t.Errorf("RandString() got length %d, want %d. String=%s", len(got), tt.nByte, got)
			}
		})
	}
}
