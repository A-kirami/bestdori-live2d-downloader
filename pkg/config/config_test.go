package config_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/A-kirami/bestdori-live2d-downloader/pkg/config"
)

func TestDefaultConfig(t *testing.T) {
	cfg := config.DefaultConfig()
	require.NotNil(t, cfg, "DefaultConfig() should not return nil")

	// 测试路径配置
	assert.Equal(t, "live2d_download", cfg.Live2dSavePath, "Live2dSavePath should be correct")
	assert.Equal(t, "live2d_chara_cache", cfg.CharaCachePath, "CharaCachePath should be correct")
	assert.Equal(t, "logs", cfg.LogPath, "LogPath should be correct")

	// 测试缓存配置
	assert.True(t, cfg.UseCharaCache, "UseCharaCache should be true")
	assert.Equal(t, 24*time.Hour, cfg.CacheDuration, "CacheDuration should be correct")

	// 测试 API 配置
	assert.Equal(t, "https://bestdori.com/assets/jp", cfg.BaseAssetsURL, "BaseAssetsURL should be correct")
	assert.Equal(t, "https://bestdori.com/api/characters", cfg.CharaRosterURL, "CharaRosterURL should be correct")
	assert.Equal(
		t,
		"https://bestdori.com/api/explorer/jp/assets/_info.json",
		cfg.AssetsIndexURL,
		"AssetsIndexURL should be correct",
	)

	// 测试下载配置
	assert.Equal(t, 20, cfg.MaxConcurrentDownloads, "MaxConcurrentDownloads should be correct")
	assert.Equal(t, 3, cfg.MaxConcurrentModels, "MaxConcurrentModels should be correct")
}

func TestInit(t *testing.T) {
	// 初始化配置
	config.Init()

	// 测试配置值是否正确
	cfg := config.Get()
	require.NotNil(t, cfg, "Get() should not return nil after Init()")

	// 测试配置值是否与默认值一致
	defaultCfg := config.DefaultConfig()
	assert.Equal(t, defaultCfg.Live2dSavePath, cfg.Live2dSavePath, "Live2dSavePath should match default")
	assert.Equal(t, defaultCfg.CharaCachePath, cfg.CharaCachePath, "CharaCachePath should match default")
	assert.Equal(t, defaultCfg.LogPath, cfg.LogPath, "LogPath should match default")
	assert.Equal(t, defaultCfg.UseCharaCache, cfg.UseCharaCache, "UseCharaCache should match default")
	assert.Equal(t, defaultCfg.CacheDuration, cfg.CacheDuration, "CacheDuration should match default")
	assert.Equal(t, defaultCfg.BaseAssetsURL, cfg.BaseAssetsURL, "BaseAssetsURL should match default")
	assert.Equal(t, defaultCfg.CharaRosterURL, cfg.CharaRosterURL, "CharaRosterURL should match default")
	assert.Equal(t, defaultCfg.AssetsIndexURL, cfg.AssetsIndexURL, "AssetsIndexURL should match default")
	assert.Equal(
		t,
		defaultCfg.MaxConcurrentDownloads,
		cfg.MaxConcurrentDownloads,
		"MaxConcurrentDownloads should match default",
	)
	assert.Equal(t, defaultCfg.MaxConcurrentModels, cfg.MaxConcurrentModels, "MaxConcurrentModels should match default")
}

func TestGet(t *testing.T) {
	cfg := config.Get()
	assert.NotNil(t, cfg, "Get() should not return nil")
	assert.NotEmpty(t, cfg.BaseAssetsURL, "BaseAssetsURL should not be empty")
	assert.NotEmpty(t, cfg.CharaRosterURL, "CharaRosterURL should not be empty")
	assert.NotEmpty(t, cfg.AssetsIndexURL, "AssetsIndexURL should not be empty")
	assert.NotEmpty(t, cfg.Live2dSavePath, "Live2dSavePath should not be empty")
	assert.NotEmpty(t, cfg.CharaCachePath, "CharaCachePath should not be empty")
	assert.Positive(t, cfg.CacheDuration, "CacheDuration should be greater than 0")
	assert.Positive(t, cfg.MaxConcurrentDownloads, "MaxConcurrentDownloads should be greater than 0")
	assert.Positive(t, cfg.MaxConcurrentModels, "MaxConcurrentModels should be greater than 0")
}
