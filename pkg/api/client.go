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
	"unicode"

	"github.com/A-kirami/bestdori-live2d-downloader/pkg/config"
	"github.com/A-kirami/bestdori-live2d-downloader/pkg/log"
	"github.com/A-kirami/bestdori-live2d-downloader/pkg/model"
	"github.com/A-kirami/bestdori-live2d-downloader/pkg/naming"
)

// Client 表示 API 客户端
// 负责处理与 Bestdori API 的所有交互.
type Client struct {
	useCharaCache  bool                                // 是否使用角色信息缓存
	charaCachePath string                              // 角色信息缓存路径
	cacheDuration  time.Duration                       // 缓存过期时间
	charaRosterURL string                              // 角色信息 API URL
	bandListURL    string                              // 乐队信息 API URL
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
		bandListURL:    cfg.BandListURL,
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
		return nil, fmt.Errorf("来自 Bestdori 的响应不是有效数据（返回了网页内容）: %s", url)
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
		if !ok || len(charaNameList) == 0 {
			continue
		}
		chineseName := selectLocalizedName(charaNameList)
		if chineseName != "" {
			names[charaID] = chineseName
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

// defaultCharaColors 没有颜色代码的角色的默认颜色映射.
//
//nolint:gochecknoglobals // 全局变量，用于存储角色默认颜色
var defaultCharaColors = map[int]string{
	601: "#DD33CC", // 奥泽美咲 (Michelle 真人形态)
}

const fallbackCharaColor = "#808080"

func resolveCharaColor(charaID int, colorCode string) string {
	if colorCode = strings.TrimSpace(colorCode); colorCode != "" {
		return colorCode
	}
	if color, ok := defaultCharaColors[charaID]; ok {
		return color
	}
	return fallbackCharaColor
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
		if !ok || len(charaNameList) == 0 {
			continue
		}

		chineseName := selectLocalizedName(charaNameList)
		if chineseName == "" {
			continue
		}

		colorCode, _ := charaInfo["colorCode"].(string)
		colorCode = resolveCharaColor(id, colorCode)
		bandID := parseBestdoriID(charaInfo["bandId"])

		result = append(result, CharacterInfo{
			ID:     id,
			BandID: bandID,
			Name:   chineseName,
			Color:  colorCode,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})

	return result, nil
}

// GetBandInfoList 获取主乐队信息列表.
func (c *Client) GetBandInfoList(ctx context.Context) ([]model.BandInfo, error) {
	data, err := c.FetchData(ctx, c.bandListURL, "band_names_1.json")
	if err != nil {
		return nil, err
	}

	return parseBandInfoList(data), nil
}

func parseBandInfoList(data map[string]any) []model.BandInfo {
	bands := make([]model.BandInfo, 0, len(data))
	for rawID, rawInfo := range data {
		id, err := strconv.Atoi(rawID)
		if err != nil || id <= 0 || id > 1000 {
			continue
		}

		info, ok := rawInfo.(map[string]any)
		if !ok {
			continue
		}
		names, ok := info["bandName"].([]any)
		if !ok {
			continue
		}
		name := normalizeBandName(selectLocalizedName(names))
		if name == "" {
			continue
		}

		bands = append(bands, model.BandInfo{ID: id, Name: name})
	}

	sort.Slice(bands, func(i, j int) bool {
		return bands[i].ID < bands[j].ID
	})
	return bands
}

func normalizeBandName(name string) string {
	const maxBandNameRunes = 64

	name = strings.TrimSpace(name)
	var normalized strings.Builder
	runeCount := 0
	for _, char := range name {
		if unicode.IsControl(char) {
			continue
		}
		if runeCount >= maxBandNameRunes {
			break
		}
		normalized.WriteRune(char)
		runeCount++
	}
	return normalized.String()
}

func parseBestdoriID(value any) int {
	number, ok := value.(float64)
	if !ok || number <= 0 || number > 1000 || number != float64(int(number)) {
		return 0
	}
	return int(number)
}

func selectLocalizedName(names []any) string {
	for _, index := range [...]int{3, 2, 0, 1} {
		if index >= len(names) {
			continue
		}
		name, ok := names[index].(string)
		if !ok {
			continue
		}
		if name = strings.TrimSpace(name); name != "" {
			return name
		}
	}
	return ""
}

func parseCostumeNameData(
	costumeData map[string]any,
) (map[string]string, map[string]string) {
	names := make(map[string]string)
	japaneseNames := make(map[string]string)

	for _, value := range costumeData {
		costume, ok := value.(map[string]any)
		if !ok {
			continue
		}
		bundleName, _ := costume["assetBundleName"].(string)
		descriptions, ok := costume["description"].([]any)
		if bundleName == "" || !ok || len(descriptions) == 0 {
			continue
		}

		if localizedName := selectLocalizedName(descriptions); localizedName != "" {
			names[bundleName] = localizedName
		}
		japaneseName, _ := descriptions[0].(string)
		if japaneseName = strings.TrimSpace(japaneseName); japaneseName != "" {
			japaneseNames[bundleName] = japaneseName
		}
	}

	return names, japaneseNames
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
	live2dAssets, err := c.getLive2dAssets(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取资源索引失败: %w", err)
	}
	for name := range live2dAssets {
		live2dNames[name] = true
	}

	return live2dNames, nil
}

// collectEventNames 获取活动名称映射（用于 event_XXX_story_YY 等模式）
// 优先级：简体中文(3) > 繁体中文(2) > 日语(0) > 英语(1).
func (c *Client) collectEventNames(ctx context.Context) map[int]string {
	eventNames := make(map[int]string)
	eventsURL := "https://bestdori.com/api/events/all.5.json"
	eventsData, eventsErr := c.FetchData(ctx, eventsURL, "events_all_5.json")
	if eventsErr == nil {
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
			if !ok || len(nameList) == 0 {
				continue
			}
			name := selectLocalizedName(nameList)
			if name != "" {
				eventNames[eventID] = name
			}
		}
	}
	return eventNames
}

// GetCostumeNames 获取服装的展示名称和多语言搜索信息
// 策略：直接用 assetBundleName 匹配，优先简体中文(index 3)，无则繁体中文(index 2)，都没有则查硬编码表
// 参数:
//   - ctx: 上下文
//
// 返回:
//   - map[string]string: Live2D服装名到展示名称的映射
//   - map[string]string: Live2D服装名到日文名称的映射
//   - error: 错误信息
//
//nolint:gocognit,gocyclo,cyclop,funlen // 复杂的多语言映射逻辑
func (c *Client) GetCostumeNames(
	ctx context.Context,
) (map[string]string, map[string]string, error) {
	costumeURL := "https://bestdori.com/api/costumes/all.5.json"
	costumeData, err := c.FetchData(ctx, costumeURL, "costume_names_5.json")
	if err != nil {
		return nil, nil, fmt.Errorf("获取服装数据失败: %w", err)
	}

	names, japaneseNames := parseCostumeNameData(costumeData)

	live2dNames, err := c.collectAllLive2dNames(ctx)
	if err != nil {
		return nil, nil, err
	}

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
			if translated := naming.TranslateCostumeSuffixWithStoryCount(
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
		if translated := naming.TranslateCostumeSuffixWithStoryCount(
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
				if japaneseName, ok := japaneseNames[live2dName]; ok && japaneseName != "" {
					names[live2dName] = chineseName + "(" + japaneseName + ")"
				}
			}
		}
	}

	return names, japaneseNames, nil
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
