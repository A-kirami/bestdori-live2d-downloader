package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/A-kirami/bestdori-live2d-downloader/pkg/config"
	"github.com/stretchr/testify/require"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

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

func TestGetCostumeNamesLoadsEachSourceOnce(t *testing.T) {
	config.Init()
	t.Cleanup(config.Init)
	cfg := config.Get()
	cfg.UseCharaCache = false
	cfg.CharaRosterURL = "https://example.test/api/characters"
	cfg.ServerTags = []string{"jp"}
	cfg.AssetServers = map[string]config.AssetServerConfig{
		"jp": {AssetsIndexURL: "https://example.test/assets.json"},
	}

	responses := map[string]string{
		"/api/costumes/all.5.json":   `{"1":{"assetBundleName":"001_casual","description":["カジュアル","Casual","休閒服","常服"]}}`,
		"/api/characters/all.5.json": `{}`,
		"/assets.json":               `{"live2d":{"chara":{"001_casual":{}}}}`,
		"/api/events/all.5.json":     `{}`,
	}
	requestCounts := make(map[string]int)
	client := NewClient()
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body, ok := responses[request.URL.Path]
		if !ok {
			return &http.Response{StatusCode: http.StatusNotFound, Body: http.NoBody}, nil
		}
		requestCounts[request.URL.Path]++
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	})}

	names, japaneseNames, err := client.GetCostumeNames(context.Background())

	require.NoError(t, err)
	require.Equal(t, "常服", names["001_casual"])
	require.Equal(t, "カジュアル", japaneseNames["001_casual"])
	require.Len(t, requestCounts, len(responses))
	for path := range responses {
		require.Equal(t, 1, requestCounts[path], path)
	}
}
