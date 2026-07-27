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
	logger, err := log.New(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, logger.Close()) })

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

func TestFilterValidCostumesReturnsAvailabilityError(t *testing.T) {
	config.Init()
	cfg := config.Get()
	cfg.AssetServers = map[string]config.AssetServerConfig{
		"jp": {BaseAssetsURL: "https://example.invalid"},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	app := &App{ctx: ctx, apiClient: api.NewClient()}

	costumes, err := app.filterValidCostumes([]model.Live2dAsset{{
		Server:  "jp",
		Costume: "001_casual",
	}})

	require.ErrorIs(t, err, context.Canceled)
	require.Empty(t, costumes)
}

func TestFilterValidCostumesKeepsAvailableResultsWhenOneCheckFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/live2d/chara/001_error_rip/buildData.asset" {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	config.Init()
	config.Get().AssetServers = map[string]config.AssetServerConfig{
		"jp": {BaseAssetsURL: server.URL},
	}
	app := &App{ctx: context.Background(), apiClient: api.NewClient()}

	costumes, err := app.filterValidCostumes([]model.Live2dAsset{
		{Server: "jp", Costume: "001_casual"},
		{Server: "jp", Costume: "001_error"},
	})

	require.Error(t, err)
	require.Equal(t, []model.Live2dAsset{{Server: "jp", Costume: "001_casual"}}, costumes)
}
