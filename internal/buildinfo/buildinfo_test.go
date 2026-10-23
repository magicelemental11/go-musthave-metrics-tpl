package buildinfo

import (
	"bytes"
	"testing"
)

func TestFprint(t *testing.T) {
	tests := []struct {
		name                  string
		version, date, commit string
		want                  string
	}{
		{
			name: "empty values",
			want: "Build version: N/A\nBuild date: N/A\nBuild commit: N/A\n",
		},
		{
			name:    "set values",
			version: "v1.0.0",
			date:    "2026-10-23T10:00:00Z",
			commit:  "abc1234",
			want:    "Build version: v1.0.0\nBuild date: 2026-10-23T10:00:00Z\nBuild commit: abc1234\n",
		},
		{
			name:    "partially set",
			version: "v1.0.0",
			want:    "Build version: v1.0.0\nBuild date: N/A\nBuild commit: N/A\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			Fprint(&buf, tt.version, tt.date, tt.commit)
			if got := buf.String(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
