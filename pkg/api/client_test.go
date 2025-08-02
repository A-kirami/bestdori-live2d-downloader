package api_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/A-kirami/bestdori-live2d-downloader/pkg/api"
	"github.com/A-kirami/bestdori-live2d-downloader/pkg/config"
	"github.com/A-kirami/bestdori-live2d-downloader/pkg/log"
	"github.com/stretchr/testify/require"
)

// setupTest 设置测试环境.
func setupTest(t *testing.T) {
	// 创建临时目录
	tempDir := t.TempDir()

	// 初始化配置
	config.Init()
	cfg := config.Get()
	cfg.LogPath = filepath.Join(tempDir, "logs")

	// 初始化日志
	if _, err := log.New(cfg.LogPath); err != nil {
		panic(fmt.Sprintf("初始化日志失败: %v", err))
	}
}

func TestMain(m *testing.M) {
	// 创建一个测试实例来设置环境
	t := &testing.T{}
	setupTest(t)
	os.Exit(m.Run())
}

func TestNewClient(t *testing.T) {
	// 创建临时目录用于测试缓存
	tempDir := t.TempDir()

	client := api.NewClient()
	client.SetCharaCachePath(tempDir)
	client.SetUseCharaCache(true)
	require.NotNil(t, client, "NewClient() should not return nil")

	// 通过实际调用API来验证客户端是否正常工作
	ctx := context.Background()
	_, err := client.FetchData(ctx, "https://bestdori.com/api/characters/all.2.json", "test_cache.json")
	require.NoError(t, err, "Client should be able to fetch data")
}

func TestFetchData(t *testing.T) {
	// 创建临时目录用于测试缓存
	tempDir := t.TempDir()

	client := api.NewClient()
	client.SetCharaCachePath(tempDir)
	client.SetUseCharaCache(true)

	tests := []struct {
		name    string
		url     string
		cache   string
		wantErr bool
	}{
		{
			name:    "有效URL",
			url:     "https://bestdori.com/api/characters/all.2.json",
			cache:   "test_cache_valid.json",
			wantErr: false,
		},
		{
			name:    "无效URL",
			url:     "http://localhost:12345/invalid.json",
			cache:   "test_cache_invalid.json",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			fetchData, fetchErr := client.FetchData(ctx, tt.url, tt.cache)

			if tt.wantErr {
				require.Error(t, fetchErr, "FetchData() should return error for invalid URL")
				require.Nil(t, fetchData, "FetchData() should return nil data for invalid URL")
			} else {
				require.NoError(t, fetchErr, "FetchData() should not return error for valid URL")
				require.NotNil(t, fetchData, "FetchData() should return non-nil data for valid URL")

				// 测试缓存
				cacheFile := filepath.Join(tempDir, tt.cache)
				_, statErr := os.Stat(cacheFile)
				require.NoError(t, statErr, "Cache file should be created")
			}
		})
	}
}

func TestGetCharaRoster(t *testing.T) {
	// 创建临时目录用于测试缓存
	tempDir := t.TempDir()

	client := api.NewClient()
	client.SetCharaCachePath(tempDir)
	client.SetUseCharaCache(true)

	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "获取角色列表",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			roster, rosterErr := client.GetCharaRoster(ctx)

			if tt.wantErr {
				require.Error(t, rosterErr, "GetCharaRoster() should return error")
				require.Nil(t, roster, "GetCharaRoster() should return nil roster")
			} else {
				require.NoError(t, rosterErr, "GetCharaRoster() should not return error")
				require.NotNil(t, roster, "GetCharaRoster() should return non-nil roster")
			}
		})
	}
}

func TestGetChara(t *testing.T) {
	// 创建临时目录用于测试缓存
	tempDir := t.TempDir()

	client := api.NewClient()
	client.SetCharaCachePath(tempDir)
	client.SetUseCharaCache(true)

	tests := []struct {
		name    string
		charaID int
		wantErr bool
	}{
		{
			name:    "有效角色ID",
			charaID: 1,
			wantErr: false,
		},
		{
			name:    "无效角色ID",
			charaID: 999999,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			chara, charaErr := client.GetChara(ctx, tt.charaID)

			if tt.wantErr {
				require.Error(t, charaErr, "GetChara() should return error for invalid ID")
				require.Nil(t, chara, "GetChara() should return nil chara for invalid ID")
			} else {
				require.NoError(t, charaErr, "GetChara() should not return error for valid ID")
				require.NotNil(t, chara, "GetChara() should return non-nil chara for valid ID")
			}
		})
	}
}

func TestValidateLive2dModel(t *testing.T) {
	// 创建临时目录用于测试缓存
	tempDir := t.TempDir()

	client := api.NewClient()
	client.SetCharaCachePath(tempDir)
	client.SetUseCharaCache(true)

	tests := []struct {
		name       string
		live2dName string
		wantExists bool
		wantErr    bool
	}{
		{
			name:       "有效的模型名称",
			live2dName: "037_casual-2023",
			wantExists: true,
			wantErr:    false,
		},
		{
			name:       "无效的模型名称",
			live2dName: "000_invalid_model",
			wantExists: false,
			wantErr:    false,
		},
		{
			name:       "空模型名称",
			live2dName: "",
			wantExists: false,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			exists, err := client.ValidateLive2dModel(ctx, tt.live2dName)

			if tt.wantErr {
				require.Error(t, err, "ValidateLive2dModel() should return error")
			} else {
				require.NoError(t, err, "ValidateLive2dModel() should not return error")
			}
			require.Equal(t, tt.wantExists, exists, "ValidateLive2dModel() should return correct existence status")
		})
	}
}
