// Package api 提供了与 Bestdori API 交互的功能
// 包括获取角色信息、Live2D 模型数据等功能
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/A-kirami/bestdori-live2d-downloader/pkg/config"
	"github.com/A-kirami/bestdori-live2d-downloader/pkg/log"
	"github.com/A-kirami/bestdori-live2d-downloader/pkg/model"
)

// Client 表示 API 客户端
// 负责处理与 Bestdori API 的所有交互.
type Client struct {
	useCharaCache  bool                                // 是否使用角色信息缓存
	charaCachePath string                              // 角色信息缓存路径
	cacheDuration  time.Duration                       // 缓存过期时间
	charaRosterURL string                              // 角色信息 API URL
	defaultServer  string                              // 默认 Bestdori 资源服务器标签
	serverTags     []string                            // Bestdori 资源服务器标签 (有序)
	assetServers   map[string]config.AssetServerConfig // Bestdori 资源服务器配置
	httpClient     *http.Client                        // HTTP 客户端
}

// NewClient 创建新的 API 客户端实例
// 返回:
//   - *Client: 新的 API 客户端实例
func NewClient() *Client {
	cfg := config.Get()
	// s, _ := cfg.AssetServers[cfg.DefaultAssetServer]
	return &Client{
		useCharaCache:  cfg.UseCharaCache,
		charaCachePath: cfg.CharaCachePath,
		cacheDuration:  cfg.CacheDuration,
		charaRosterURL: cfg.CharaRosterURL,
		defaultServer:  cfg.DefaultAssetServer,
		serverTags:     cfg.ServerTags,
		assetServers:   cfg.AssetServers,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// readCacheData 从缓存文件读取数据
// 参数:
//   - cacheFile: 缓存文件路径
//
// 返回:
//   - map[string]any: 缓存数据
//   - error: 错误信息
func (c *Client) readCacheData(cacheFile string) (map[string]any, error) {
	cacheData, readErr := os.ReadFile(cacheFile)
	if readErr != nil {
		log.DefaultLogger.Error().Str("cacheFile", cacheFile).Err(readErr).Msg("读取缓存数据失败")
		return nil, fmt.Errorf("读取缓存数据失败: %w", readErr)
	}

	var result map[string]any
	if unmarshalErr := json.Unmarshal(cacheData, &result); unmarshalErr != nil {
		log.DefaultLogger.Error().Str("cacheFile", cacheFile).Err(unmarshalErr).Msg("解析缓存数据失败")
		return nil, fmt.Errorf("解析缓存数据失败: %w", unmarshalErr)
	}

	return result, nil
}

// FetchData 从指定 URL 获取数据，支持缓存功能
// 参数:
//   - ctx: 上下文
//   - url: 请求的 URL
//   - cache: 缓存文件名（为空则不使用缓存）
//
// 返回:
//   - map[string]any: 获取的数据
//   - error: 错误信息
func (c *Client) FetchData(ctx context.Context, url string, cache string) (map[string]any, error) {
	if c.useCharaCache && cache != "" {
		cacheFile := filepath.Join(c.charaCachePath, cache)
		if fileInfo, err := os.Stat(cacheFile); err == nil {
			// 检查文件修改时间是否在缓存期限内
			if time.Since(fileInfo.ModTime()) < c.cacheDuration {
				log.DefaultLogger.Info().Str("cacheFile", cacheFile).Msg("使用缓存数据")
				return c.readCacheData(cacheFile)
			}
			log.DefaultLogger.Info().Str("cacheFile", cacheFile).Msg("缓存已过期")
		}
	}

	log.DefaultLogger.Info().Str("url", url).Msg("开始获取数据")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		log.DefaultLogger.Error().Str("url", url).Err(err).Msg("创建请求失败")
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// #nosec G704 -- 请求的 URL 源自配置或受控输入，已在调用方或配置中验证
	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.DefaultLogger.Error().Str("url", url).Err(err).Msg("获取数据失败")
		return nil, fmt.Errorf("获取数据失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.DefaultLogger.Error().Str("url", url).Int("statusCode", resp.StatusCode).Msg("HTTP错误")
		return nil, fmt.Errorf("HTTP错误: %d", resp.StatusCode)
	}

	var result map[string]any
	if decodeErr := json.NewDecoder(resp.Body).Decode(&result); decodeErr != nil {
		log.DefaultLogger.Error().Str("url", url).Err(decodeErr).Msg("解析JSON失败")
		return nil, fmt.Errorf("解析JSON失败: %w", decodeErr)
	}

	if c.useCharaCache && cache != "" {
		if mkdirErr := os.MkdirAll(c.charaCachePath, 0750); mkdirErr != nil {
			log.DefaultLogger.Error().Str("path", c.charaCachePath).Err(mkdirErr).Msg("创建缓存目录失败")
			return nil, fmt.Errorf("创建缓存目录失败: %w", mkdirErr)
		}
		if jsonData, marshalErr := json.Marshal(result); marshalErr == nil {
			cacheFilePath := filepath.Join(c.charaCachePath, cache)
			if writeErr := os.WriteFile(cacheFilePath, jsonData, 0600); writeErr != nil {
				log.DefaultLogger.Error().Str("cacheFile", cacheFilePath).Err(writeErr).Msg("写入缓存文件失败")
				return nil, fmt.Errorf("写入缓存文件失败: %w", writeErr)
			}
			log.DefaultLogger.Info().Str("cacheFile", cacheFilePath).Msg("缓存数据已保存")
		}
	}

	log.DefaultLogger.Info().Str("url", url).Msg("数据获取成功")
	return result, nil
}

// GetCharaRoster 获取所有角色信息列表
// 参数:
//   - ctx: 上下文
//
// 返回:
//   - map[string]any: 角色信息列表
//   - error: 错误信息
func (c *Client) GetCharaRoster(ctx context.Context) (map[string]any, error) {
	url := fmt.Sprintf("%s/all.2.json", c.charaRosterURL)
	return c.FetchData(ctx, url, "chara_roster.json")
}

// GetChara 获取指定角色的详细信息
// 参数:
//   - ctx: 上下文
//   - charaID: 角色ID
//
// 返回:
//   - map[string]any: 角色详细信息
//   - error: 错误信息
func (c *Client) GetChara(ctx context.Context, charaID int) (map[string]any, error) {
	url := fmt.Sprintf("%s/%d.json", c.charaRosterURL, charaID)
	return c.FetchData(ctx, url, fmt.Sprintf("chara_%d.json", charaID))
}

// getSingleLive2dAssets 获取单个资源服务器的 Live2D 资源映射
// 参数:
//   - ctx: 上下文
//   - tag: 服务器标签
//   - s: Bestdori 资源服务器
//
// 返回:
//   - map[string]any: Live2D 资源映射
//   - error: 错误信息
func (c *Client) getSingleLive2dAssets(
	ctx context.Context,
	tag string,
	s *config.AssetServerConfig,
) (map[string]any, error) {
	assetsInfo, err := c.FetchData(ctx, s.AssetsIndexURL, fmt.Sprintf("assets_info_%s.json", tag))
	if err != nil {
		return nil, err
	}

	live2dAssets, ok := assetsInfo["live2d"].(map[string]any)["chara"].(map[string]any)
	if !ok {
		return nil, errors.New("无效的资源索引格式")
	}

	return live2dAssets, nil
}

// getLive2dAssets 获取 Live2D 资源映射
// 参数:
//   - ctx: 上下文
//
// 返回:
//   - map[string]string: Live2D 资源及所属服务器
//   - error: 错误信息
func (c *Client) getLive2dAssets(ctx context.Context) (map[string]string, error) {
	live2dAssets := make(map[string]string)

	for _, tag := range c.serverTags { // 有序
		s, o := c.assetServers[tag]
		if !o {
			return nil, fmt.Errorf("未定义的 Bestdori 服务器标签: %s", s)
		}

		newLive2dAssets, err := c.getSingleLive2dAssets(ctx, tag, &s)
		if err != nil {
			return nil, err
		}

		for costume := range newLive2dAssets {
			if _, exists := live2dAssets[costume]; !exists {
				live2dAssets[costume] = tag
			}
		}
	}

	return live2dAssets, nil
}

func isCharaCostume(costume string, charaID int) bool {
	parts := strings.Split(costume, "_")
	if len(parts) < 2 {
		return false
	}

	idStr := fmt.Sprintf("%03d", charaID)
	if parts[0] == idStr {
		return true
	}

	return len(parts) >= 3 && parts[0] == "bili" && parts[1] == idStr
}

// GetCharaCostumes 获取指定角色的所有 Live2D 服装列表
// 参数:
//   - ctx: 上下文
//   - charaID: 角色ID
//
// 返回:
//   - []string: 服装列表（按特定规则排序）
//   - error: 错误信息
func (c *Client) GetCharaCostumes(ctx context.Context, charaID int) ([]model.Live2dAsset, error) {
	live2dAssets, err := c.getLive2dAssets(ctx)
	if err != nil {
		return nil, err
	}

	var costumes []model.Live2dAsset
	for costume, server := range live2dAssets {
		if isCharaCostume(costume, charaID) && !strings.HasSuffix(costume, "general") {
			costumes = append(costumes, model.Live2dAsset{
				Server:  server,
				Costume: costume,
			})
		}
	}

	// 对服装列表进行排序 (使用 model 包中的比较函数)
	sort.Slice(costumes, func(i, j int) bool {
		return model.CostumeLess(costumes[i], costumes[j])
	})

	return costumes, nil
}

// GetLive2dData 获取指定 Live2D 模型的构建数据
// 参数:
//   - ctx: 上下文
//   - live2d: Live2D 资源
//
// 返回:
//   - *config.AssetServerConfig: 模型所属 Bestdori 服务器 API 信息
//   - *model.BuildData: Live2D 构建数据
//   - error: 错误信息
func (c *Client) GetLive2dData(
	ctx context.Context,
	live2d *model.Live2dAsset,
) (*config.AssetServerConfig, *model.BuildData, error) {
	// 获取模型所属服务器的配置
	s, o := c.assetServers[live2d.Server]
	if !o {
		return nil, nil, fmt.Errorf("不存在的服务器来源: %s", live2d.Server)
	}

	// 构建资源包 URL
	url := fmt.Sprintf("%s/live2d/chara/%s_rip/buildData.asset", s.BaseAssetsURL, live2d.Costume)
	log.DefaultLogger.Info().Str("live2dName", live2d.Costume).Str("url", url).Msg("开始获取Live2D构建数据")

	// 获取构建数据
	data, err := c.FetchData(ctx, url, "")
	if err != nil {
		log.DefaultLogger.Error().Str("live2dName", live2d.Costume).Err(err).Msg("获取构建数据失败")
		return nil, nil, fmt.Errorf("获取构建数据失败: %w", err)
	}

	// 提取基础数据
	baseData, ok := data["Base"].(map[string]any)
	if !ok {
		log.DefaultLogger.Error().Str("live2dName", live2d.Costume).Msg("构建数据格式错误: 缺少 Base 字段")
		return nil, nil, errors.New("构建数据格式错误: 缺少 Base 字段")
	}

	// 序列化基础数据
	jsonData, err := json.Marshal(baseData)
	if err != nil {
		log.DefaultLogger.Error().Str("live2dName", live2d.Costume).Err(err).Msg("序列化构建数据失败")
		return nil, nil, fmt.Errorf("序列化构建数据失败: %w", err)
	}

	// 反序列化为 BuildData 结构
	var buildData model.BuildData
	if unmarshalErr := json.Unmarshal(jsonData, &buildData); unmarshalErr != nil {
		log.DefaultLogger.Error().Str("live2dName", live2d.Costume).Err(unmarshalErr).Msg("反序列化构建数据失败")
		return nil, nil, fmt.Errorf("反序列化构建数据失败: %w", unmarshalErr)
	}

	// 处理 model 和 motions 文件的 .bytes 后缀
	buildData.Model.RemoveBytesSuffix()
	for i := range buildData.Motions {
		buildData.Motions[i].RemoveBytesSuffix()
	}

	// 确保纹理文件名有 .png 后缀
	for i := range buildData.Textures {
		buildData.Textures[i].EnsurePngSuffix()
	}

	log.DefaultLogger.Info().Str("live2dName", live2d.Costume).Msg("Live2D构建数据处理完成")
	return &s, &buildData, nil
}

// GetLive2dAsset 获取指定模型所属的资源信息
// 参数:
//   - ctx: 上下文
//   - live2dName: Live2D 模型名称
//
// 返回:
//   - *model.Live2dAsset: 模型资源信息
//   - bool: 模型是否存在
//   - error: 错误信息
func (c *Client) GetLive2dAsset(
	ctx context.Context,
	live2dName string,
) (*model.Live2dAsset, bool, error) {
	live2dAssets, err := c.getLive2dAssets(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("获取资源索引失败: %w", err)
	}

	server, exists := live2dAssets[live2dName]
	if !exists {
		return nil, false, nil
	}

	return &model.Live2dAsset{
		Server:  server,
		Costume: live2dName,
	}, true, nil
}

// ValidateLive2dModel 验证指定的 Live2D 模型是否存在
// 参数:
//   - ctx: 上下文
//   - live2dName: Live2D 模型名称
//
// 返回:
//   - bool: 模型是否存在
//   - error: 错误信息
func (c *Client) ValidateLive2dModel(ctx context.Context, live2dName string) (bool, error) {
	_, exists, err := c.GetLive2dAsset(ctx, live2dName)
	if err != nil {
		return false, err
	}

	return exists, nil
}

// SetCharaCachePath 设置角色信息缓存路径
// 参数:
//   - path: 缓存路径
func (c *Client) SetCharaCachePath(path string) {
	c.charaCachePath = path
}

// SetUseCharaCache 设置是否使用角色信息缓存
// 参数:
//   - use: 是否使用缓存
func (c *Client) SetUseCharaCache(use bool) {
	c.useCharaCache = use
}

// GetDefaultAssetServer 获取默认 Bestdori 服务器标签
// 返回:
//   - string: 默认 Bestdori 服务器标签
func (c *Client) GetDefaultAssetServer() string {
	if c.defaultServer != "" {
		return c.defaultServer
	}

	return c.serverTags[0]
}
