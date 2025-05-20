package utils_test

import (
	"testing"

	"github.com/A-kirami/bestdori-live2d-downloader/pkg/utils"
	"github.com/stretchr/testify/assert"
)

func TestFindBestMatch(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		candidates map[string][]string
		wantID     string
		wantName   string
	}{
		{
			name:  "完全匹配-中文",
			query: "千早爱音",
			candidates: map[string][]string{
				"37": {"千早 愛音", "Anon Chihaya", "千早 愛音", "千早 爱音"},
			},
			wantID:   "37",
			wantName: "千早 爱音",
		},
		{
			name:  "完全匹配-英文",
			query: "Anon Chihaya",
			candidates: map[string][]string{
				"37": {"千早 愛音", "Anon Chihaya", "千早 愛音", "千早 爱音"},
			},
			wantID:   "37",
			wantName: "Anon Chihaya",
		},
		{
			name:  "部分匹配-名字",
			query: "爱音",
			candidates: map[string][]string{
				"37": {"千早 愛音", "Anon Chihaya", "千早 愛音", "千早 爱音"},
			},
			wantID:   "37",
			wantName: "千早 爱音",
		},
		{
			name:  "无匹配",
			query: "不存在",
			candidates: map[string][]string{
				"37": {"千早 愛音", "Anon Chihaya", "千早 愛音", "千早 爱音"},
			},
			wantID:   "",
			wantName: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotID, gotName, _ := utils.FindBestMatch(tt.query, tt.candidates)
			assert.Equal(t, tt.wantID, gotID, "ID should match")
			assert.Equal(t, tt.wantName, gotName, "Name should match")
		})
	}
}
