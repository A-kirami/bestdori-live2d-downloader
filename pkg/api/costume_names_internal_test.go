package api

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSelectLocalizedName(t *testing.T) {
	tests := []struct {
		name         string
		descriptions []any
		want         string
	}{
		{
			name:         "simplified Chinese first",
			descriptions: []any{"日语", "English", "繁體", " 简体 "},
			want:         "简体",
		},
		{
			name:         "blank simplified Chinese falls back to traditional Chinese",
			descriptions: []any{"日语", "English", " 繁體 ", "   "},
			want:         "繁體",
		},
		{
			name:         "missing Chinese falls back to Japanese",
			descriptions: []any{" 日语 ", "English"},
			want:         "日语",
		},
		{
			name:         "blank preferred languages fall back to English",
			descriptions: []any{" ", " English ", nil, "\t"},
			want:         "English",
		},
		{
			name:         "all blank",
			descriptions: []any{" ", "\t", nil, "\n"},
			want:         "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, selectLocalizedName(tt.descriptions))
		})
	}
}
