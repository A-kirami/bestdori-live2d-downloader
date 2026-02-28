// Package config 提供了程序的配置管理功能
package config

import (
	"fmt"
	"time"
)

// AssetServerConfig 表示 Bestdori 资源服务器配置.
type AssetServerConfig struct {
	// API 配置
	BaseAssetsURL  string // Bestdori 资源基础 URL
	AssetsIndexURL string // 资源索引 API URL
}

// Config 表示程序的配置结构.
type Config struct {
	// 路径配置
	Live2dSavePath string // Live2D 模型保存路径
	CharaCachePath string // 角色信息缓存路径
	LogPath        string // 日志文件保存路径

	// 缓存配置
	UseCharaCache bool          // 是否使用角色信息缓存
	CacheDuration time.Duration // 缓存过期时间

	// API 配置
	CharaRosterURL string // 角色信息 API URL
	// DefaultAssetServer 指定默认使用的资源服务器（例如 "jp"）
	DefaultAssetServer string
	ServerTags         []string                     // Bestdori 资源服务器标签 (有序)
	AssetServers       map[string]AssetServerConfig // Bestdori 资源服务器

	// 下载配置
	MaxConcurrentDownloads int // 单个模型下载时的最大并发文件下载数
	MaxConcurrentModels    int // 最大并发模型下载数
}

var (
	// 全局配置实例.
	//nolint:gochecknoglobals // 使用全局配置实例是必要的，因为需要在程序的不同部分访问相同的配置
	globalConfig *Config
)

func DefaultAssetServers() []string {
	return []string{"jp", "cn", "en", "kr", "tw"}
}

func DefaultAssetServerConfigTemplate(s string) AssetServerConfig {
	return AssetServerConfig{
		BaseAssetsURL:  fmt.Sprintf("https://bestdori.com/assets/%s", s),
		AssetsIndexURL: fmt.Sprintf("https://bestdori.com/api/explorer/%s/assets/_info.json", s),
	}
}

// DefaultAssetServersConfig 返回默认 Bestdori 资源服务器配置.
func DefaultAssetServersConfig() map[string]AssetServerConfig {
	serverConfigs := make(map[string]AssetServerConfig)
	for _, s := range DefaultAssetServers() {
		serverConfigs[s] = DefaultAssetServerConfigTemplate(s)
	}
	return serverConfigs
}

// DefaultConfig 返回默认配置.
func DefaultConfig() *Config {
	return &Config{
		// 路径配置
		Live2dSavePath: "live2d_download",
		CharaCachePath: "live2d_chara_cache",
		LogPath:        "logs",

		// 缓存配置
		UseCharaCache: true,
		CacheDuration: 24 * time.Hour,

		// API 配置
		CharaRosterURL:     "https://bestdori.com/api/characters",
		DefaultAssetServer: "jp",
		ServerTags:         DefaultAssetServers(),
		AssetServers:       DefaultAssetServersConfig(),

		// 下载配置
		MaxConcurrentDownloads: 20,
		MaxConcurrentModels:    3,
	}
}

// Init 初始化全局配置.
func Init() {
	globalConfig = DefaultConfig()
}

// Get 获取全局配置实例.
func Get() *Config {
	if globalConfig == nil {
		Init()
	}
	return globalConfig
}
