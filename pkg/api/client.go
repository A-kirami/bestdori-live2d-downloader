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
	"regexp"
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
//
//nolint:gocognit // 复杂的数据获取和缓存逻辑
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

	// 检查 Content-Type，避免将 HTML 错误页面当作 JSON 解析
	contentType := resp.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "text/html") {
		log.DefaultLogger.Error().Str("url", url).Str("contentType", contentType).Msg("返回了HTML而非JSON")
		return nil, fmt.Errorf("资源不存在或无法访问: %s", url)
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

// GetCharacterNames 获取所有角色的中文名称映射
// 返回 map[characterID]chineseFullName
// 参数:
//   - ctx: 上下文
//
// 返回:
//   - map[string]string: 角色ID到中文全名的映射
//   - error: 错误信息
//
//nolint:gocognit // 多语言优先级处理固有复杂度
func (c *Client) GetCharacterNames(ctx context.Context) (map[string]string, error) {
	url := fmt.Sprintf("%s/all.5.json", c.charaRosterURL)
	data, err := c.FetchData(ctx, url, "chara_names_5.json")
	if err != nil {
		return nil, err
	}

	names := make(map[string]string)
	for charaID, info := range data {
		charaInfo, ok := info.(map[string]any)
		if !ok {
			continue
		}
		// 使用 characterName（全名）而非 firstName
		charaNameList, ok := charaInfo["characterName"].([]any)
		if !ok || len(charaNameList) < 4 {
			continue
		}
		// 优先级：简体中文(3) > 繁体中文(2) > 日语(0) > 英语(1)
		chineseName := ""
		if len(charaNameList) > 3 {
			chineseName, _ = charaNameList[3].(string)
		}
		if chineseName == "" && len(charaNameList) > 2 {
			chineseName, _ = charaNameList[2].(string)
		}
		if chineseName == "" && len(charaNameList) > 0 {
			chineseName, _ = charaNameList[0].(string)
		}
		if chineseName == "" && len(charaNameList) > 1 {
			chineseName, _ = charaNameList[1].(string)
		}
		if chineseName != "" {
			names[charaID] = strings.TrimSpace(chineseName)
		}
	}

	// 特殊角色：米歇尔/奥泽美咲 (ID 15)
	if name, ok := names["15"]; ok {
		names["15"] = name + "/奥泽 美咲"
	}

	return names, nil
}

// CharacterInfo 表示角色信息.
type CharacterInfo = model.CharacterInfo

// 服装类型常量.
const (
	costumePajamas   = "睡衣"
	costumeHalloween = "万圣节"
	costumePractice  = "剧情初始服装"
)

// defaultCharaColors 没有颜色代码的角色的默认颜色映射.
//
//nolint:gochecknoglobals // 全局变量，用于存储角色默认颜色
var defaultCharaColors = map[int]string{
	601: "#DD33CC", // 奥泽美咲 (Michelle 真人形态)
}

// GetCharacterInfoList 获取所有角色信息列表（包含颜色）
// 参数:
//   - ctx: 上下文
//
// 返回:
//   - []CharacterInfo: 角色信息列表
//   - error: 错误信息
func (c *Client) GetCharacterInfoList(ctx context.Context) ([]CharacterInfo, error) {
	url := fmt.Sprintf("%s/all.5.json", c.charaRosterURL)
	data, err := c.FetchData(ctx, url, "chara_names_5.json")
	if err != nil {
		return nil, err
	}

	var result []CharacterInfo
	for charaID, info := range data {
		id, parseErr := strconv.Atoi(charaID)
		if parseErr != nil || id > 1000 {
			continue
		}

		charaInfo, ok := info.(map[string]any)
		if !ok {
			continue
		}

		// 使用 characterName（全名）而不是 firstName
		charaNameList, ok := charaInfo["characterName"].([]any)
		if !ok || len(charaNameList) < 4 {
			continue
		}

		chineseName, _ := charaNameList[3].(string)
		if chineseName == "" {
			continue
		}
		chineseName = strings.TrimSpace(chineseName)

		colorCode, _ := charaInfo["colorCode"].(string)

		// 为没有颜色的角色添加硬编码颜色
		if colorCode == "" {
			if fallback, hasDefault := defaultCharaColors[id]; hasDefault {
				colorCode = fallback
			}
		}

		result = append(result, CharacterInfo{
			ID:    id,
			Name:  chineseName,
			Color: colorCode,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})

	return result, nil
}

// CostumeNameInfo 表示服装的多语言名称信息.
type CostumeNameInfo struct {
	Original string // 原始名称（assetBundleName）
	Chinese  string // 中文名称（简体/繁体）
	Japanese string // 日文名称
	English  string // 英文名称
}

// collectAllLive2dNames 收集所有 Live2D 名称
// 从角色 API 的 seasonCostumeListMap 和资源索引中收集.
//
//nolint:gocognit // 遍历嵌套 JSON 结构
func (c *Client) collectAllLive2dNames(ctx context.Context) (map[string]bool, error) {
	charaURL := fmt.Sprintf("%s/all.5.json", c.charaRosterURL)
	charaData, err := c.FetchData(ctx, charaURL, "chara_names_5.json")
	if err != nil {
		return nil, fmt.Errorf("获取角色数据失败: %w", err)
	}

	live2dNames := make(map[string]bool)
	for charaIDStr, info := range charaData {
		charaID, parseErr := strconv.Atoi(charaIDStr)
		if parseErr != nil || charaID > 1000 {
			continue
		}
		charaInfo, charaOk := info.(map[string]any)
		if !charaOk {
			continue
		}
		seasonMap, seasonMapOk := charaInfo["seasonCostumeListMap"].(map[string]any)
		if !seasonMapOk {
			continue
		}
		entries, entriesOk := seasonMap["entries"].(map[string]any)
		if !entriesOk {
			continue
		}
		for _, season := range entries {
			seasonData, seasonOk := season.(map[string]any)
			if !seasonOk {
				continue
			}
			costumeEntries, costumeOk := seasonData["entries"].([]any)
			if !costumeOk {
				continue
			}
			for _, entry := range costumeEntries {
				entryData, entryOk := entry.(map[string]any)
				if !entryOk {
					continue
				}
				live2dName, _ := entryData["live2dAssetBundleName"].(string)
				if live2dName != "" {
					live2dNames[live2dName] = true
				}
			}
		}
	}

	// 也从资源索引获取所有 Live2D 服装名
	live2dAssets, _ := c.getLive2dAssets(ctx)
	for name := range live2dAssets {
		live2dNames[name] = true
	}

	return live2dNames, nil
}

// collectEventNames 获取活动名称映射（用于 event_XXX_story_YY 等模式）
// 优先级：简体中文(3) > 繁体中文(2) > 日语(0) > 英语(1).
//
//nolint:gocognit // 多语言优先级选择固有复杂度
func (c *Client) collectEventNames(ctx context.Context) map[int]string {
	eventNames := make(map[int]string)
	eventsURL := "https://bestdori.com/api/events/all.5.json"
	eventsData, eventsErr := c.FetchData(ctx, eventsURL, "events_all_5.json")
	if eventsErr == nil { //nolint:nestif // 复杂的事件数据处理
		for eventIDStr, info := range eventsData {
			eventID, parseErr := strconv.Atoi(eventIDStr)
			if parseErr != nil {
				continue
			}
			eventInfo, ok := info.(map[string]any)
			if !ok {
				continue
			}
			nameList, ok := eventInfo["eventName"].([]any)
			if !ok || len(nameList) < 4 {
				continue
			}
			name := ""
			if len(nameList) > 3 {
				name, _ = nameList[3].(string)
			}
			if name == "" && len(nameList) > 2 {
				name, _ = nameList[2].(string)
			}
			if name == "" && len(nameList) > 0 {
				name, _ = nameList[0].(string)
			}
			if name == "" && len(nameList) > 1 {
				name, _ = nameList[1].(string)
			}
			if name != "" {
				eventNames[eventID] = strings.TrimSpace(name)
			}
		}
	}
	return eventNames
}

// GetCostumeNameInfo 获取所有服装的多语言名称信息
// 返回 map[live2dAssetBundleName]CostumeNameInfo
// 参数:
//   - ctx: 上下文
//
// 返回:
//   - map[string]*CostumeNameInfo: Live2D服装名到多语言名称信息的映射
//   - error: 错误信息
//
//nolint:gocognit // 复杂的多语言映射逻辑
func (c *Client) GetCostumeNameInfo(ctx context.Context) (map[string]*CostumeNameInfo, error) {
	costumeURL := "https://bestdori.com/api/costumes/all.5.json"
	costumeData, err := c.FetchData(ctx, costumeURL, "costume_names_5.json")
	if err != nil {
		return nil, fmt.Errorf("获取服装数据失败: %w", err)
	}

	// 从服装 API 建立 assetBundleName -> 名称信息
	names := make(map[string]*CostumeNameInfo)
	for _, info := range costumeData {
		costumeInfo, ok := info.(map[string]any)
		if !ok {
			continue
		}
		bundleName, _ := costumeInfo["assetBundleName"].(string)
		if bundleName == "" {
			continue
		}
		descList, ok := costumeInfo["description"].([]any)
		if !ok || len(descList) < 1 {
			continue
		}

		nameInfo := &CostumeNameInfo{Original: bundleName}

		// 获取各语言描述
		if len(descList) > 0 {
			nameInfo.Japanese, _ = descList[0].(string)
			nameInfo.Japanese = strings.TrimSpace(nameInfo.Japanese)
		}
		if len(descList) > 1 {
			nameInfo.English, _ = descList[1].(string)
			nameInfo.English = strings.TrimSpace(nameInfo.English)
		}
		if len(descList) > 2 {
			nameInfo.Chinese, _ = descList[2].(string) // 繁体中文
			nameInfo.Chinese = strings.TrimSpace(nameInfo.Chinese)
		}
		if len(descList) > 3 {
			if simplified, _ := descList[3].(string); simplified != "" {
				nameInfo.Chinese = strings.TrimSpace(simplified) // 简体中文优先
			}
		}

		names[bundleName] = nameInfo
	}

	live2dNames, err := c.collectAllLive2dNames(ctx)
	if err != nil {
		return nil, err
	}

	eventNames := c.collectEventNames(ctx)

	// 为没有直接匹配的 Live2D 名补充硬编码映射
	for live2dName := range live2dNames {
		if _, exists := names[live2dName]; exists {
			continue
		}
		// 提取后缀
		parts := strings.SplitN(live2dName, "_", 2)
		if len(parts) < 2 {
			continue
		}
		suffix := parts[1]

		// 使用模式匹配翻译（传入 eventNames）
		if translated := translateCostumeSuffix(suffix, eventNames); translated != "" {
			names[live2dName] = &CostumeNameInfo{
				Original: live2dName,
				Chinese:  translated,
			}
		}
	}

	return names, nil
}

// GetCostumeNames 获取所有服装的中文名称映射
// 返回 map[live2dAssetBundleName]chineseDescription
// 策略：直接用 assetBundleName 匹配，优先简体中文(index 3)，无则繁体中文(index 2)，都没有则查硬编码表
// 参数:
//   - ctx: 上下文
//
// 返回:
//   - map[string]string: Live2D服装名到中文描述的映射
//   - error: 错误信息
//
//nolint:gocognit,gocyclo,cyclop,funlen // 复杂的多语言映射逻辑
func (c *Client) GetCostumeNames(ctx context.Context) (map[string]string, error) {
	costumeURL := "https://bestdori.com/api/costumes/all.5.json"
	costumeData, err := c.FetchData(ctx, costumeURL, "costume_names_5.json")
	if err != nil {
		return nil, fmt.Errorf("获取服装数据失败: %w", err)
	}

	// 从服装 API 建立 assetBundleName -> 中文描述
	// 同时收集日语名称，用于后续中文名冲突消歧
	names := make(map[string]string)
	jpnNames := make(map[string]string)
	for _, info := range costumeData {
		costumeInfo, ok := info.(map[string]any)
		if !ok {
			continue
		}
		bundleName, _ := costumeInfo["assetBundleName"].(string)
		if bundleName == "" {
			continue
		}
		descList, ok := costumeInfo["description"].([]any)
		if !ok || len(descList) < 3 {
			continue
		}
		// 收集日语名称（用于同名消歧）
		if len(descList) > 0 {
			if jpn, _ := descList[0].(string); jpn != "" {
				jpnNames[bundleName] = strings.TrimSpace(jpn)
			}
		}
		// 优先级：简体中文(3) > 繁体中文(2) > 日语(0) > 英语(1)
		desc := ""
		if len(descList) > 3 {
			desc, _ = descList[3].(string)
		}
		if desc == "" && len(descList) > 2 {
			desc, _ = descList[2].(string)
		}
		if desc == "" && len(descList) > 0 {
			desc, _ = descList[0].(string)
		}
		if desc == "" && len(descList) > 1 {
			desc, _ = descList[1].(string)
		}
		if desc != "" {
			names[bundleName] = strings.TrimSpace(desc)
		}
	}

	live2dNames, err := c.collectAllLive2dNames(ctx)
	if err != nil {
		return nil, err
	}

	// 为没有直接匹配的 Live2D 名补充通用服装映射

	eventNames := c.collectEventNames(ctx)

	// 预计算每个角色+活动的剧情编号数量
	charaEventStoryNumbers := make(map[string]map[string]bool)
	storyWithNumRe := regexp.MustCompile(`^event_?(\d+)_story_?(\w+)$`)
	storyNoNumRe := regexp.MustCompile(`^event_?(\d+)_story$`)
	for live2dName := range live2dNames {
		proc := strings.TrimPrefix(live2dName, "bili_")
		parts := strings.SplitN(proc, "_", 2)
		if len(parts) < 2 {
			continue
		}
		charaID := parts[0]
		suffix := parts[1]
		if m := storyWithNumRe.FindStringSubmatch(suffix); len(m) > 2 { //nolint:nestif // 复杂的剧情编号统计逻辑
			if _, parseErr := strconv.Atoi(m[1]); parseErr == nil {
				num := strings.TrimLeft(m[2], "0")
				if num == "" {
					num = "1"
				}
				key := charaID + ":" + m[1]
				if charaEventStoryNumbers[key] == nil {
					charaEventStoryNumbers[key] = make(map[string]bool)
				}
				charaEventStoryNumbers[key][num] = true
			}
		} else if n := storyNoNumRe.FindStringSubmatch(suffix); len(n) > 1 {
			if _, parseErr := strconv.Atoi(n[1]); parseErr == nil {
				key := charaID + ":" + n[1]
				if charaEventStoryNumbers[key] == nil {
					charaEventStoryNumbers[key] = make(map[string]bool)
				}
				charaEventStoryNumbers[key]["1"] = true
			}
		}
	}
	charaEventStoryCounts := make(map[string]int)
	for key, nums := range charaEventStoryNumbers {
		charaEventStoryCounts[key] = len(nums)
	}

	for live2dName := range live2dNames {
		// 处理 bili_ 前缀
		nameToProcess := strings.TrimPrefix(live2dName, "bili_")
		parts := strings.SplitN(nameToProcess, "_", 2)
		if len(parts) < 2 {
			continue
		}
		suffix := parts[1]

		// event_XXX_story_YY 模式：始终使用活动翻译（覆盖 API 卡牌描述）
		if storyWithNumRe.MatchString(suffix) || storyNoNumRe.MatchString(suffix) {
			if translated := translateCostumeSuffixWithStoryCount(
				suffix,
				eventNames,
				charaEventStoryCounts,
				parts[0],
			); translated != "" {
				if apiName, ok := names[live2dName]; ok && apiName != translated {
					names[live2dName] = translated + "(" + apiName + ")"
				} else {
					names[live2dName] = translated
				}
				continue
			}
		}

		if _, exists := names[live2dName]; exists {
			continue
		}

		// 年份后缀 API 映射查找
		if yearMatch := regexp.MustCompile(`^(.+)-(\d{4})$`).FindStringSubmatch(suffix); len(yearMatch) > 2 {
			baseSuffix := yearMatch[1]
			year := yearMatch[2]
			baseLive2dName := parts[0] + "_" + baseSuffix
			if baseName, ok := names[baseLive2dName]; ok && baseName != "" {
				names[live2dName] = fmt.Sprintf("%s(%s)", baseName, year)
				continue
			}
		}

		// 使用模式匹配翻译
		if translated := translateCostumeSuffixWithStoryCount(
			suffix,
			eventNames,
			charaEventStoryCounts,
			parts[0],
		); translated != "" {
			names[live2dName] = translated
		}
	}

	// 检测并解决中文名称冲突：同一角色内重名时在末尾追加原始日语名以消歧
	// 不同角色可以有同名服装（路径为 角色名/服装名），无需消歧.
	// 解决例如 013_live_event_198_ssr 和 013_live_sr_01 都翻译为"大家一起来！"时
	// 下载路径冲突导致模型被误判为重复而跳过的问题.
	type nameEntry struct {
		live2dName  string
		chineseName string
	}
	charaGroups := make(map[string][]nameEntry)
	for live2dName, chineseName := range names {
		// 提取角色 ID：格式为 charaID_suffix 或 bili_charaID_suffix
		charaID := live2dName
		charaID = strings.TrimPrefix(charaID, "bili_")
		if idx := strings.Index(charaID, "_"); idx > 0 {
			charaID = charaID[:idx]
		}
		charaGroups[charaID] = append(charaGroups[charaID], nameEntry{live2dName, chineseName})
	}
	for _, entries := range charaGroups {
		nameCount := make(map[string][]string)
		for _, e := range entries {
			nameCount[e.chineseName] = append(nameCount[e.chineseName], e.live2dName)
		}
		for chineseName, live2dNamesList := range nameCount {
			if len(live2dNamesList) <= 1 {
				continue
			}
			for _, live2dName := range live2dNamesList {
				if jpnName, ok := jpnNames[live2dName]; ok && jpnName != "" {
					names[live2dName] = chineseName + "(" + jpnName + ")"
				}
			}
		}
	}

	return names, nil
}

// translateVariant 翻译服装变体后缀.
func translateVariant(variant string) string {
	variants := map[string]string{
		"penlight": "荧光棒",
		"nocap":    "无帽",
		"sunglass": "墨镜",
	}
	if name, ok := variants[variant]; ok {
		return name
	}
	return variant
}

// translateCostumeSuffixWithStoryCount 带剧情数量信息的翻译.
// 按角色+活动统计，单剧情不加编号，多剧情始终加编号.
//
//nolint:gocognit,nestif // 复杂的剧情翻译逻辑
func translateCostumeSuffixWithStoryCount(
	suffix string,
	eventNames map[int]string,
	charaEventStoryCounts map[string]int,
	charaID string,
) string {
	// 活动剧情：event_XXX_story_YY
	if matches := regexp.MustCompile(`^event_?(\d+)_story_?(\w+)$`).FindStringSubmatch(suffix); len(matches) > 2 {
		eventID, err := strconv.Atoi(matches[1])
		if err == nil {
			storyNum := matches[2]
			storyNum = strings.TrimLeft(storyNum, "0")
			if storyNum == "" {
				storyNum = "1"
			}
			key := charaID + ":" + matches[1]
			if charaEventStoryCounts[key] > 1 {
				if eventName, ok := eventNames[eventID]; ok {
					return fmt.Sprintf("%s·剧情%s", eventName, storyNum)
				}
				return fmt.Sprintf("活动%s·剧情%s", matches[1], storyNum)
			}
			if eventName, ok := eventNames[eventID]; ok {
				return fmt.Sprintf("%s·剧情", eventName)
			}
			return fmt.Sprintf("活动%s·剧情", matches[1])
		}
	}

	// 活动剧情（无编号）：event_XXX_story
	if matches := regexp.MustCompile(`^event_?(\d+)_story$`).FindStringSubmatch(suffix); len(matches) > 1 {
		eventID, err := strconv.Atoi(matches[1])
		if err == nil {
			key := charaID + ":" + matches[1]
			if charaEventStoryCounts[key] > 1 {
				if eventName, ok := eventNames[eventID]; ok {
					return fmt.Sprintf("%s·剧情1", eventName)
				}
				return fmt.Sprintf("活动%s·剧情1", matches[1])
			}
			if eventName, ok := eventNames[eventID]; ok {
				return fmt.Sprintf("%s·剧情", eventName)
			}
			return fmt.Sprintf("活动%s·剧情", matches[1])
		}
	}

	return translateCostumeSuffix(suffix, eventNames)
}

// translateCostumeSuffix 根据后缀模式翻译服装名称
// 优先级：精确匹配 > 模式匹配 > 年份组合匹配
//
//nolint:gocognit,gocyclo,cyclop,funlen // 复杂的模式匹配逻辑
func translateCostumeSuffix(suffix string, eventNames map[int]string) string {
	// 1. 精确匹配表
	exactMatches := map[string]string{
		// 基础通用服
		"casual":                 "常服",
		"casual_summer":          "夏季常服",
		"casual_winter":          "冬季常服",
		"casual_winter-sunglass": "常服(冬·墨镜)",
		"casual_v3":              "常服v3",
		"school":                 "校服",
		"school_winter":          "冬服",
		"school_winter_2":        "冬服2",
		"school_winter_s2":       "冬服s2",
		"school_winter_v3":       "冬服v3",
		"school_summer":          "夏服",
		"school_summer_s2":       "夏服s2",
		"school_summer_v3":       "夏服v3",
		"school_armband":         "校服(臂章)",
		"jh_school_winter":       "初中冬服",
		"uniform":                "制服",
		"default":                "默认",
		"general":                "通用",

		// 场景/职业/运动服
		"gym_clothes":            "体操服",
		"tracksuit":              "运动服",
		"pajamas":                costumePajamas,
		"swimsuit":               "泳装",
		"swim_swit":              "泳装",
		"apron":                  "围裙",
		"cafe":                   "咖啡厅制服",
		"fast_food":              "快餐店制服",
		"arbeit":                 "打工服",
		"store":                  "店员服",
		"chairperson_casual":     "学生会长服",
		"practice_clothes":       costumePractice,
		"stage_costume":          "舞台服装",
		"wd_practice":            "白色情人节练习服",
		"garupa_t":               "Garupa T恤",
		"memorial_middle_school": "纪念中学制服",

		// 节日/纪念服
		"af":                        "愚人节",
		"xmas":                      "圣诞",
		"hw":                        costumeHalloween,
		"halloween":                 costumeHalloween,
		"halloween_without_lantern": "万圣节(无灯)",
		"furisode":                  "振袖",
		"yukata":                    "浴衣",
		"anniv":                     "周年纪念",
		"special_5th":               "5周年纪念",
		"girlparty2019":             "女子聚会2019",
		"precious_summer":           "珍贵夏日",
		"anime_live":                "动画Live",
		"popipa_fes":                "Popipa祭典",
		"kirameki_festival":         "闪光祭典",
		"kirameki_festival_coat":    "闪光祭典(外套)",

		// 剧情/舞台服
		"chapter0_live":    "序章Live",
		"chapter0_pajamas": "序章睡衣",
		"romeo":            "罗密欧",
		"juliet":           "朱丽叶",

		// 特殊联动/企划
		"michelle":        "米歇尔(兔子玩偶服)",
		"michelle_ranger": "米歇尔·游侠(兔子玩偶服)",
		"miko":            "巫女服",
		"fantasy":         "Neo Fantasy Online",
		"fantasy_01":      "Neo Fantasy Online 01",
		"delta":           "Delta变体服",
		"expose":          "《EXPOSE》演出服",
		"ranger":          "游侠服",
		"chispa":          "CHiSPA乐队服",
		"sumimi":          "Sumimi企划服",
		"nfo01":           "《NFO》游戏服装01",
		"boss":            "Boss服",
		"robot":           "机器人服",

		// 系统基础服
		"live_default":      "初始打歌服",
		"live_practice":     costumePractice,
		"live_sr_01":        "Live SR",
		"live_ssr_01":       "Live SSR",
		"vocal_limited_sr":  "Vocal限定SR",
		"vocal_limited_ssr": "Vocal限定SSR",
	}

	if name, ok := exactMatches[suffix]; ok {
		return name
	}

	// 2. 模式匹配（使用正则）

	// 愚人节：YYYYaf 或 YYYY_af
	if matches := regexp.MustCompile(`^(\d{4})af$`).FindStringSubmatch(suffix); len(matches) > 1 {
		return matches[1] + "愚人节"
	}

	// 年份+基础服装+变体：如 casual-2023-penlight, casual-2023-nocap
	if matches := regexp.MustCompile(`^(.+)-(\d{4})-(.+)$`).FindStringSubmatch(suffix); len(matches) > 3 {
		baseName := translateCostumeSuffix(matches[1], eventNames)
		if baseName != "" {
			variantName := translateVariant(matches[3])
			return fmt.Sprintf("%s(%s)(%s)", baseName, matches[2], variantName)
		}
	}

	// 年份+基础服装：如 casual-2023, school_winter-2022
	if matches := regexp.MustCompile(`^(.+)-(\d{4})$`).FindStringSubmatch(suffix); len(matches) > 2 {
		baseName := translateCostumeSuffix(matches[1], eventNames)
		if baseName != "" {
			return fmt.Sprintf("%s(%s)", baseName, matches[2])
		}
	}

	// 年份前缀+基础服装：如 2024_furisode, 2019_furisode
	if matches := regexp.MustCompile(`^(\d{4})_(.+)$`).FindStringSubmatch(suffix); len(matches) > 2 {
		baseName := translateCostumeSuffix(matches[2], eventNames)
		if baseName != "" {
			return fmt.Sprintf("%s(%s)", baseName, matches[1])
		}
	}

	// 章节：chapterX_live, chapterX_pajamas
	if matches := regexp.MustCompile(`^chapter(\d+)_(.+)$`).FindStringSubmatch(suffix); len(matches) > 2 {
		chapterNum := matches[1]
		costumeType := matches[2]
		var typeName string
		switch costumeType {
		case "live":
			typeName = "Live"
		case costumePajamas:
			typeName = "睡衣"
		default:
			typeName = costumeType
		}
		if chapterNum == "0" {
			return "序章" + typeName
		}
		return fmt.Sprintf("第%s章%s", chapterNum, typeName)
	}

	// 活动服装：event_XXX（无 _story_ 后缀）
	if matches := regexp.MustCompile(`^event_?(\d+)$`).FindStringSubmatch(suffix); len(matches) > 1 {
		eventID, err := strconv.Atoi(matches[1])
		if err == nil {
			if eventName, ok := eventNames[eventID]; ok {
				return eventName
			}
			return fmt.Sprintf("活动%s", matches[1])
		}
	}

	// 活动剧情：event_XXX_story_YY 或 eventXXX_storyYY
	if matches := regexp.MustCompile(`^event_?(\d+)_story_?(\w+)$`).FindStringSubmatch(suffix); len(matches) > 2 {
		eventID, err := strconv.Atoi(matches[1])
		if err == nil {
			if eventName, ok := eventNames[eventID]; ok {
				return fmt.Sprintf("%s(剧情%s)", eventName, matches[2])
			}
			return fmt.Sprintf("活动%s(剧情%s)", matches[1], matches[2])
		}
	}

	// 活动卡牌：live_event_XXX_稀有度
	if matches := regexp.MustCompile(`^live_event_(\d+)_(r|sr|ssr|ur)$`).FindStringSubmatch(suffix); len(matches) > 2 {
		eventID, err := strconv.Atoi(matches[1])
		if err == nil {
			if eventName, ok := eventNames[eventID]; ok {
				return fmt.Sprintf("%s(%s)", eventName, strings.ToUpper(matches[2]))
			}
			return fmt.Sprintf("活动%s(%s)", matches[1], strings.ToUpper(matches[2]))
		}
	}

	// 联动：collabo_XXX
	if strings.HasPrefix(suffix, "collabo") {
		return "联动服"
	}

	// 梦想祭：dream_festival_XXX
	if strings.HasPrefix(suffix, "dream_festival") {
		return "梦想祭"
	}

	// 生日：birthday_XXX
	if strings.HasPrefix(suffix, "birthday") {
		return "生日限定"
	}

	// 总选举：Xth_general_election_r 或 general_election
	if strings.Contains(suffix, "general_election") {
		if matches := regexp.MustCompile(`(\d+).*general_election`).FindStringSubmatch(suffix); len(matches) > 1 {
			return fmt.Sprintf("第%s届总选举", matches[1])
		}
		return "总选举"
	}

	// 乐队故事：band_story_X
	if matches := regexp.MustCompile(`^band_story_(\d+)$`).FindStringSubmatch(suffix); len(matches) > 1 {
		return fmt.Sprintf("乐队故事%s", matches[1])
	}

	// 特定故事：story_XX 或 story_XX-YYYY
	if matches := regexp.MustCompile(`^story_(\d+)(-.*)?$`).FindStringSubmatch(suffix); len(matches) > 1 {
		if matches[1] == "01" || matches[1] == "1" {
			return costumePractice
		}
		if matches[1] == "03" {
			return "米歇尔玩偶服"
		}
		return ""
	}

	// 初音联动：miku_XXX
	if strings.HasPrefix(suffix, "miku_") {
		mikuSongs := map[string]string{
			"miku_shinkai":      "深海少女",
			"miku_migikata":     "右肩之蝶",
			"miku_rettou":       "左肩之蝶",
			"miku_alien":        "Alien Alien",
			"miku_lostone":      "Lost One的号哭",
			"miku_romecin":      "罗密欧与辛德瑞拉",
			"miku_nocturnality": "夜之蝶",
		}
		if name, ok := mikuSongs[suffix]; ok {
			return "初音联动·" + name
		}
		return "初音联动"
	}

	// 角色专属：other-XX
	if matches := regexp.MustCompile(`other-(\d+)$`).FindStringSubmatch(suffix); len(matches) > 1 {
		// other-12 是活动小丑
		if matches[1] == "12" {
			return "活动小丑"
		}
		// other-41 是愚人节
		if matches[1] == "41" {
			return "愚人节"
		}
		return fmt.Sprintf("角色专属%s", matches[1])
	}

	// 振袖年份：2019_furisode 等（已在上面处理）

	// 愚人节变体：2021af 等（已在上面处理）

	// 3. 尝试从硬编码表中查找基础部分
	baseCostumes := map[string]string{
		"casual":      "常服",
		"school":      "校服",
		"pajamas":     costumePajamas,
		"swimsuit":    "泳装",
		"yukata":      "浴衣",
		"halloween":   costumeHalloween,
		"christmas":   "圣诞",
		"furisode":    "振袖",
		"arbeit":      "打工服",
		"expose":      "《EXPOSE》演出服",
		"fantasy":     "Neo Fantasy Online",
		"delta":       "Delta变体服",
		"miko":        "巫女服",
		"apron":       "围裙",
		"store":       "店员服",
		"tracksuit":   "运动服",
		"garupa":      "Garupa",
		"popipa":      "Popipa",
		"anime":       "动画",
		"michelle":    "米歇尔(兔子玩偶服)",
		"ranger":      "游侠服",
		"cafe":        "咖啡厅",
		"fast_food":   "快餐",
		"gym":         "体操",
		"chairperson": "学生会长",
		"memorial":    "纪念",
		"stage":       "舞台",
		"practice":    "练习",
		"live":        "Live",
		"fes":         "祭典",
		"boss":        "Boss服",
		"robot":       "机器人服",
		"chispa":      "CHiSPA乐队服",
		"sumimi":      "Sumimi企划服",
		"nfo":         "《NFO》游戏服装",
		"wd":          "白色情人节",
	}

	// 尝试匹配基础词
	for baseKey, baseName := range baseCostumes {
		if strings.Contains(suffix, baseKey) {
			return baseName
		}
	}

	return ""
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

// GetLive2dAssets 获取 Live2D 资源映射（公开方法）.
func (c *Client) GetLive2dAssets(ctx context.Context) (map[string]string, error) {
	return c.getLive2dAssets(ctx)
}

// GetAssetServer 获取指定服务器的配置.
func (c *Client) GetAssetServer(tag string) (config.AssetServerConfig, bool) {
	s, ok := c.assetServers[tag]
	return s, ok
}

// HTTPClient 获取 HTTP 客户端.
func (c *Client) HTTPClient() *http.Client {
	return c.httpClient
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
