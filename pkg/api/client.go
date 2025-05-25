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
	"strconv"
	"strings"
	"time"

	"github.com/A-kirami/bestdori-live2d-downloader/pkg/config"
	"github.com/A-kirami/bestdori-live2d-downloader/pkg/log"
	"github.com/A-kirami/bestdori-live2d-downloader/pkg/model"
)

// Client 表示 API 客户端
// 负责处理与 Bestdori API 的所有交互.
type Client struct {
	useCharaCache  bool          // 是否使用角色信息缓存
	charaCachePath string        // 角色信息缓存路径
	cacheDuration  time.Duration // 缓存过期时间
	baseAssetsURL  string        // Bestdori 资源基础 URL
	charaRosterURL string        // 角色信息 API URL
	assetsIndexURL string        // 资源索引 API URL
	httpClient     *http.Client  // HTTP 客户端
}

// NewClient 创建新的 API 客户端实例
// 返回:
//   - *Client: 新的 API 客户端实例
func NewClient() *Client {
	cfg := config.Get()
	return &Client{
		useCharaCache:  cfg.UseCharaCache,
		charaCachePath: cfg.CharaCachePath,
		cacheDuration:  cfg.CacheDuration,
		baseAssetsURL:  cfg.BaseAssetsURL,
		charaRosterURL: cfg.CharaRosterURL,
		assetsIndexURL: cfg.AssetsIndexURL,
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

// GetCharaCostumes 获取指定角色的所有 Live2D 服装列表
// 参数:
//   - ctx: 上下文
//   - charaID: 角色ID
//
// 返回:
//   - []string: 服装列表（按特定规则排序）
//   - error: 错误信息
func (c *Client) GetCharaCostumes(ctx context.Context, charaID int) ([]string, error) {
	assetsInfo, err := c.FetchData(ctx, c.assetsIndexURL, "assets_info.json")
	if err != nil {
		return nil, err
	}

	live2dAssets, ok := assetsInfo["live2d"].(map[string]any)["chara"].(map[string]any)
	if !ok {
		return nil, errors.New("无效的资源索引格式")
	}

	var costumes []string
	for live2d := range live2dAssets {
		if live2d[:3] == fmt.Sprintf("%03d", charaID) && !strings.HasSuffix(live2d, "general") {
			costumes = append(costumes, live2d)
		}
	}

	// 对服装列表进行排序
	sort.Slice(costumes, func(i, j int) bool {
		// 提取服装ID（模型名称中的数字部分）
		iParts := strings.Split(costumes[i], "_")
		jParts := strings.Split(costumes[j], "_")

		// 如果包含"live_event"，将其排在后面
		iHasEvent := strings.Contains(costumes[i], "live_event")
		jHasEvent := strings.Contains(costumes[j], "live_event")

		if iHasEvent != jHasEvent {
			return !iHasEvent
		}

		// 比较服装ID
		if len(iParts) > 1 && len(jParts) > 1 {
			iID, iErr := strconv.Atoi(iParts[1])
			jID, jErr := strconv.Atoi(jParts[1])
			if iErr == nil && jErr == nil {
				return iID < jID
			}
		}

		// 如果无法比较ID，则按字符串排序
		return costumes[i] < costumes[j]
	})

	return costumes, nil
}

// GetLive2dData 获取指定 Live2D 模型的构建数据
// 参数:
//   - ctx: 上下文
//   - live2dName: Live2D 模型名称
//
// 返回:
//   - *model.BuildData: Live2D 构建数据
//   - error: 错误信息
func (c *Client) GetLive2dData(ctx context.Context, live2dName string) (*model.BuildData, error) {
	// 构建资源包 URL
	url := fmt.Sprintf("%s/live2d/chara/%s_rip/buildData.asset", c.baseAssetsURL, live2dName)
	log.DefaultLogger.Info().Str("live2dName", live2dName).Str("url", url).Msg("开始获取Live2D构建数据")

	// 获取构建数据
	data, err := c.FetchData(ctx, url, "")
	if err != nil {
		log.DefaultLogger.Error().Str("live2dName", live2dName).Err(err).Msg("获取构建数据失败")
		return nil, fmt.Errorf("获取构建数据失败: %w", err)
	}

	// 提取基础数据
	baseData, ok := data["Base"].(map[string]any)
	if !ok {
		log.DefaultLogger.Error().Str("live2dName", live2dName).Msg("构建数据格式错误: 缺少 Base 字段")
		return nil, errors.New("构建数据格式错误: 缺少 Base 字段")
	}

	// 序列化基础数据
	jsonData, err := json.Marshal(baseData)
	if err != nil {
		log.DefaultLogger.Error().Str("live2dName", live2dName).Err(err).Msg("序列化构建数据失败")
		return nil, fmt.Errorf("序列化构建数据失败: %w", err)
	}

	// 反序列化为 BuildData 结构
	var buildData model.BuildData
	if unmarshalErr := json.Unmarshal(jsonData, &buildData); unmarshalErr != nil {
		log.DefaultLogger.Error().Str("live2dName", live2dName).Err(unmarshalErr).Msg("反序列化构建数据失败")
		return nil, fmt.Errorf("反序列化构建数据失败: %w", unmarshalErr)
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

	log.DefaultLogger.Info().Str("live2dName", live2dName).Msg("Live2D构建数据处理完成")
	return &buildData, nil
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
