package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/A-kirami/bestdori-live2d-downloader/pkg/config"
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

func TestCollectAllLive2dNamesReturnsAssetIndexError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/characters/all.5.json" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("{}"))
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(server.Close)

	config.Init()
	t.Cleanup(config.Init)
	cfg := config.Get()
	cfg.UseCharaCache = false
	cfg.CharaRosterURL = server.URL + "/characters"
	cfg.ServerTags = []string{"jp"}
	cfg.AssetServers = map[string]config.AssetServerConfig{
		"jp": {AssetsIndexURL: server.URL + "/assets.json"},
	}

	client := NewClient()
	_, err := client.collectAllLive2dNames(context.Background())

	require.ErrorContains(t, err, "获取资源索引失败")
}
