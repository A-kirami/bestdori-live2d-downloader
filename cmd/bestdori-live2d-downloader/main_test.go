package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/A-kirami/bestdori-live2d-downloader/pkg/api"
	"github.com/A-kirami/bestdori-live2d-downloader/pkg/config"
	"github.com/A-kirami/bestdori-live2d-downloader/pkg/log"
	"github.com/stretchr/testify/require"
)

func setupDirectDownloadTestClient(
	t *testing.T,
	assetServers map[string]map[string]any,
) *api.Client {
	t.Helper()

	config.Init()
	cfg := config.Get()
	logDir, err := os.MkdirTemp("", "bestdori-main-test-logs-*")
	require.NoError(t, err)
	cfg.LogPath = logDir
	cfg.ServerTags = make([]string, 0, len(assetServers))
	cfg.AssetServers = make(map[string]config.AssetServerConfig, len(assetServers))
	_, err = log.New(cfg.LogPath)
	require.NoError(t, err)

	mux := http.NewServeMux()
	for tag, costumes := range assetServers {
		cfg.ServerTags = append(cfg.ServerTags, tag)
		cfg.AssetServers[tag] = config.AssetServerConfig{
			BaseAssetsURL:  "https://example.invalid/assets/" + tag,
			AssetsIndexURL: "http://example.invalid/" + tag + "/assets/_info.json",
		}

		response := map[string]any{
			"live2d": map[string]any{
				"chara": costumes,
			},
		}

		path := "/" + tag + "/assets/_info.json"
		mux.HandleFunc(path, func(w http.ResponseWriter, _ *http.Request) {
			require.NoError(t, json.NewEncoder(w).Encode(response))
		})
	}

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	for tag, serverCfg := range cfg.AssetServers {
		serverCfg.AssetsIndexURL = server.URL + "/" + tag + "/assets/_info.json"
		cfg.AssetServers[tag] = serverCfg
	}

	client := api.NewClient()
	client.SetUseCharaCache(false)
	client.SetCharaCachePath(t.TempDir())
	return client
}

func TestResolveDirectDownloadAssetsUsesResolvedServer(t *testing.T) {
	client := setupDirectDownloadTestClient(t, map[string]map[string]any{
		"jp": {},
		"cn": {
			"037_casual-2023": map[string]any{},
		},
	})
	app := &App{
		ctx:       context.Background(),
		apiClient: client,
	}

	assets, invalidModels, err := app.resolveDirectDownloadAssets([]string{"037_casual-2023"})

	require.NoError(t, err)
	require.Empty(t, invalidModels)
	require.Len(t, assets, 1)
	require.Equal(t, "cn", assets[0].Server)
	require.Equal(t, "037_casual-2023", assets[0].Costume)
}
