package utils

import (
	"strings"
	"testing"
)

func TestReadersEqual(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		left      string
		right     string
		chunkSize int
		wantEqual bool
	}{
		{
			name:      "same content",
			left:      "abcdef",
			right:     "abcdef",
			chunkSize: 2,
			wantEqual: true,
		},
		{
			name:      "different content",
			left:      "abcdef",
			right:     "abcxef",
			chunkSize: 2,
			wantEqual: false,
		},
		{
			name:      "different lengths",
			left:      "abc",
			right:     "abcd",
			chunkSize: 2,
			wantEqual: false,
		},
		{
			name:      "default chunk size",
			left:      strings.Repeat("z", 5000),
			right:     strings.Repeat("z", 5000),
			chunkSize: 0,
			wantEqual: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			equal, err := ReadersEqual(strings.NewReader(tc.left), strings.NewReader(tc.right), tc.chunkSize)
			if err != nil {
				t.Fatalf("ReadersEqual() error = %v", err)
			}
			if equal != tc.wantEqual {
				t.Fatalf("ReadersEqual() = %v, want %v", equal, tc.wantEqual)
			}
		})
	}
}
