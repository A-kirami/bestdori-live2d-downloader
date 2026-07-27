package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/A-kirami/bestdori-live2d-downloader/pkg/api"
	"github.com/A-kirami/bestdori-live2d-downloader/pkg/config"
	"github.com/A-kirami/bestdori-live2d-downloader/pkg/log"
	"github.com/A-kirami/bestdori-live2d-downloader/pkg/model"
	"github.com/stretchr/testify/require"
)

func setupDirectDownloadTestClient(
	t *testing.T,
	assetServers map[string]map[string]any,
) *api.Client {
	t.Helper()

	config.Init()
	cfg := config.Get()
	cfg.LogPath = t.TempDir()
	cfg.ServerTags = make([]string, 0, len(assetServers))
	cfg.AssetServers = make(map[string]config.AssetServerConfig, len(assetServers))
	logger, err := log.New(cfg.LogPath)
	require.NoError(t, err)
	t.Cleanup(func() {
		if closeErr := logger.Close(); closeErr != nil {
			t.Errorf("close logger: %v", closeErr)
		}
	})

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
			if encodeErr := json.NewEncoder(w).Encode(response); encodeErr != nil {
				t.Errorf("encode %s response: %v", path, encodeErr)
			}
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

func TestShouldHandleAsDirectDownloadSupportsBiliSpecialModels(t *testing.T) {
	client := setupDirectDownloadTestClient(t, map[string]map[string]any{
		"jp": {
			"bili_001_collabo_r": map[string]any{},
		},
	})
	app := &App{
		ctx:       context.Background(),
		apiClient: client,
	}

	direct, err := app.shouldHandleAsDirectDownload("bili_001_collabo_r")

	require.NoError(t, err)
	require.True(t, direct)
}

func TestShouldHandleAsDirectDownloadSupportsRegularModelNames(t *testing.T) {
	client := setupDirectDownloadTestClient(t, map[string]map[string]any{
		"jp": {
			"037_casual-2023": map[string]any{},
		},
	})
	app := &App{
		ctx:       context.Background(),
		apiClient: client,
	}

	direct, err := app.shouldHandleAsDirectDownload("037_casual-2023")

	require.NoError(t, err)
	require.True(t, direct)
}

func TestShouldHandleAsDirectDownloadFallsBackForPlainText(t *testing.T) {
	client := setupDirectDownloadTestClient(t, map[string]map[string]any{
		"jp": {
			"037_casual-2023": map[string]any{},
		},
	})
	app := &App{
		ctx:       context.Background(),
		apiClient: client,
	}

	direct, err := app.shouldHandleAsDirectDownload("kasumi")

	require.NoError(t, err)
	require.False(t, direct)
}

func TestHasCompleteModelFindsOtherNamingModeWithoutMovingIt(t *testing.T) {
	config.Init()
	savePath := t.TempDir()
	config.Get().Live2dSavePath = savePath

	chinesePath := filepath.Join(savePath, "户山香澄", "常服")
	require.NoError(t, os.MkdirAll(filepath.Join(chinesePath, "data"), 0750))
	require.NoError(t, os.WriteFile(filepath.Join(chinesePath, "data", "model.moc"), []byte("model"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(chinesePath, "model.json"), []byte("{}"), 0600))

	app := &App{
		ctx:          context.Background(),
		charaNames:   map[string]string{"1": "户山香澄"},
		costumeNames: map[string]string{"001_casual": "常服"},
		costumeNameInfo: map[string]*api.CostumeNameInfo{
			"001_casual": {Chinese: "常服"},
		},
	}
	originalPath := filepath.Join(savePath, "Kasumi Toyama", "001_casual")

	require.True(t, app.hasCompleteModel("001_casual", originalPath, &model.BuildData{}))
	require.NoDirExists(t, originalPath)
	require.DirExists(t, chinesePath)
}

func TestGetLive2dPathOriginalPreservesAssetName(t *testing.T) {
	config.Init()
	savePath := t.TempDir()
	config.Get().Live2dSavePath = savePath

	app := &App{charaNames: map[string]string{"1": "户山香澄"}}
	path, err := app.getLive2dPathOriginal("001_casual", 1)

	require.NoError(t, err)
	require.Equal(t, filepath.Join(savePath, "户山香澄", "001_casual"), path)
}

func TestLoadCharacterNamesInitializesOnceConcurrently(t *testing.T) {
	logger, err := log.New(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, logger.Close())
	})

	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		response := map[string]any{
			"1": map[string]any{
				"characterName": []string{"Kasumi", "Kasumi Toyama", "戶山香澄", "户山香澄"},
			},
		}
		if encodeErr := json.NewEncoder(w).Encode(response); encodeErr != nil {
			t.Errorf("encode character response: %v", encodeErr)
		}
	}))
	t.Cleanup(server.Close)

	config.Init()
	cfg := config.Get()
	cfg.CharaRosterURL = server.URL
	cfg.UseCharaCache = false
	client := api.NewClient()
	app := &App{ctx: context.Background(), apiClient: client}

	var waitGroup sync.WaitGroup
	for range 20 {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			app.loadCharacterNames()
		}()
	}
	waitGroup.Wait()

	require.Equal(t, int32(1), requestCount.Load())
	require.Equal(t, "户山香澄", app.charaNames["1"])
}
