package downloader_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/A-kirami/bestdori-live2d-downloader/pkg/api"
	"github.com/A-kirami/bestdori-live2d-downloader/pkg/config"
	"github.com/A-kirami/bestdori-live2d-downloader/pkg/downloader"
	"github.com/A-kirami/bestdori-live2d-downloader/pkg/log"
	"github.com/A-kirami/bestdori-live2d-downloader/pkg/model"

	"github.com/stretchr/testify/assert"
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

func TestNewDownloader(t *testing.T) {
	apiClient := api.NewClient()
	downloader := downloader.NewDownloader(apiClient, nil, nil)
	require.NotNil(t, downloader, "NewDownloader() should not return nil")
	assert.NotNil(t, downloader.GetAPIClient(), "NewDownloader() apiClient should not be nil")
}

func TestDownloadBundleFile(t *testing.T) {
	// 创建临时目录用于测试下载
	tempDir := t.TempDir()

	apiClient := api.NewClient()
	downloader := downloader.NewDownloader(apiClient, nil, nil)

	tests := []struct {
		name       string
		bundleFile model.BundleFile
		filePath   string
		wantErr    bool
	}{
		{
			name: "有效文件",
			bundleFile: model.BundleFile{
				BundleName: "live2d/chara/037_general",
				FileName:   "texture_00.png",
			},
			filePath: filepath.Join(tempDir, "texture_00.png"),
			wantErr:  false,
		},
		{
			name: "无效文件",
			bundleFile: model.BundleFile{
				BundleName: "live2d/chara/invalid_bundle_name",
				FileName:   "invalid.txt",
			},
			filePath: filepath.Join(tempDir, "invalid.txt"),
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			downloadErr := downloader.DownloadBundleFile(ctx, tt.bundleFile, tt.filePath, false)

			if tt.wantErr {
				require.Error(t, downloadErr, "DownloadBundleFile() should return error for invalid file")
				_, statErr := os.Stat(tt.filePath)
				require.True(t, os.IsNotExist(statErr), "File should not exist for invalid download")
			} else {
				require.NoError(t, downloadErr, "DownloadBundleFile() should not return error for valid file")
				_, readErr := os.Stat(tt.filePath)
				require.NoError(t, readErr, "Downloaded file should exist")
			}
		})
	}
}

func TestLive2dBuilder(t *testing.T) {
	// 创建临时目录用于测试构建
	tempDir := t.TempDir()

	apiClient := api.NewClient()
	d := downloader.NewDownloader(apiClient, nil, nil)

	// 创建测试文件
	testFiles := []string{
		"data/model.moc",
		"data/physics.json",
		"data/textures/texture_00.png",
		"data/textures/texture_01.png",
		"data/motions/idle01.mtn",
		"data/expressions/default.exp.json",
	}
	for _, file := range testFiles {
		filePath := filepath.Join(tempDir, file)
		mkdirErr := os.MkdirAll(filepath.Dir(filePath), 0755)
		require.NoError(t, mkdirErr, "Failed to create directory for %s", file)
		writeErr := os.WriteFile(filePath, []byte("test"), 0644)
		require.NoError(t, writeErr, "Failed to create test file %s", file)
	}

	tests := []struct {
		name      string
		path      string
		buildData *model.BuildData
		wantErr   bool
	}{
		{
			name: "有效构建数据",
			path: tempDir,
			buildData: &model.BuildData{
				Model: model.BundleFile{
					BundleName: "live2d/chara/037_casual-2023",
					FileName:   "model.moc",
				},
				Physics: model.BundleFile{
					BundleName: "live2d/chara/037_casual-2023",
					FileName:   "physics.json",
				},
				Textures: []model.BundleFile{
					{
						BundleName: "live2d/chara/037_general",
						FileName:   "texture_00.png",
					},
					{
						BundleName: "live2d/chara/037_casual-2023",
						FileName:   "texture_01.png",
					},
				},
				Transition: model.BundleFile{
					BundleName: "live2d/chara/037_general",
					FileName:   "anonTransitionData.asset",
				},
				Motions: []model.BundleFile{
					{
						BundleName: "live2d/chara/037_general",
						FileName:   "idle01.mtn",
					},
				},
				Expressions: []model.BundleFile{
					{
						BundleName: "live2d/chara/037_general",
						FileName:   "default.exp.json",
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := downloader.NewLive2dBuilder(tt.path, tt.buildData, d, "test_model")
			require.NotNil(t, builder, "NewLive2dBuilder() should not return nil")

			constructErr := builder.Construct()
			if tt.wantErr {
				require.Error(t, constructErr, "Live2dBuilder.Construct() should return error")
			} else {
				require.NoError(t, constructErr, "Live2dBuilder.Construct() should not return error")

				// 检查必要的目录和文件是否创建
				expectedDirs := []string{
					"data",
					"data/textures",
					"data/motions",
					"data/expressions",
				}
				for _, dir := range expectedDirs {
					dirPath := filepath.Join(tt.path, dir)
					_, dirStatErr := os.Stat(dirPath)
					require.NoError(t, dirStatErr, "Directory %s should exist", dir)
				}

				// 检查 model.json 是否创建
				modelJSON := filepath.Join(tt.path, "model.json")
				_, jsonStatErr := os.Stat(modelJSON)
				require.NoError(t, jsonStatErr, "model.json should exist")
			}
		})
	}
}
