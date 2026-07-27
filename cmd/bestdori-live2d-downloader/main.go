// Package main 是 Bestdori Live2D 下载器的主程序包
// 该程序用于从 Bestdori 网站选择并下载 Live2D 模型
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/A-kirami/bestdori-live2d-downloader/pkg/api"
	"github.com/A-kirami/bestdori-live2d-downloader/pkg/config"
	"github.com/A-kirami/bestdori-live2d-downloader/pkg/downloader"
	"github.com/A-kirami/bestdori-live2d-downloader/pkg/log"
	"github.com/A-kirami/bestdori-live2d-downloader/pkg/model"
	"github.com/A-kirami/bestdori-live2d-downloader/pkg/tui"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	// ErrDownloadCancelled 表示下载已取消的错误.
	ErrDownloadCancelled = "下载已取消"
)

// App 表示应用程序的主要结构.
type App struct {
	ctx              context.Context
	cancel           context.CancelFunc
	apiClient        *api.Client
	dl               *downloader.Downloader
	tuiModel         *tui.Model
	program          *tea.Program
	charaNames       map[string]string               // 角色ID -> 中文名
	costumeNames     map[string]string               // 服装资源包名 -> 中文描述
	costumeNameInfo  map[string]*api.CostumeNameInfo // 服装资源包名 -> 多语言名称信息
	charaNamesOnce   sync.Once
	costumeNamesOnce sync.Once
}

// NewApp 创建新的应用程序实例.
func NewApp() *App {
	ctx, cancel := context.WithCancel(context.Background())
	return &App{
		ctx:    ctx,
		cancel: cancel,
	}
}

// initialize 初始化应用程序.
func (a *App) initialize() {
	// 初始化配置
	config.Init()
	cfg := config.Get()

	// 初始化日志
	if _, err := log.New(cfg.LogPath); err != nil {
		log.DefaultLogger.Error().Err(err).Msg("初始化日志失败")
		os.Exit(1)
	}

	// 创建 TUI 模型
	model := tui.NewModel()
	a.tuiModel = &model
	a.program = tea.NewProgram(a.tuiModel, tea.WithAltScreen())
	a.tuiModel.SetProgram(a.program)

	// 创建 API 客户端和下载器
	a.apiClient = api.NewClient()
	a.dl = downloader.NewDownloader(a.apiClient, a.tuiModel, a.program)
}

// loadCharacterNames 加载角色中文名称映射.
func (a *App) loadCharacterNames() {
	a.charaNamesOnce.Do(func() {
		if a.charaNames != nil {
			return
		}

		names, err := a.apiClient.GetCharacterNames(a.ctx)
		if err != nil {
			log.DefaultLogger.Warn().Err(err).Msg("加载角色中文名失败，将使用原始名称")
			a.charaNames = make(map[string]string)
			return
		}

		a.charaNames = names
		log.DefaultLogger.Info().Int("count", len(names)).Msg("加载角色中文名成功")
	})
}

func (a *App) localizedCharacterName(charaID int) string {
	a.loadCharacterNames()
	return a.charaNames[strconv.Itoa(charaID)]
}

// loadCostumeNames 加载服装名称映射.
func (a *App) loadCostumeNames() {
	a.costumeNamesOnce.Do(func() {
		if a.costumeNames != nil && a.costumeNameInfo != nil {
			return
		}

		// 加载中文名称映射（用于显示和文件夹命名）
		names, err := a.apiClient.GetCostumeNames(a.ctx)
		if err != nil {
			log.DefaultLogger.Warn().Err(err).Msg("加载服装中文名失败，将使用原始名称")
			a.costumeNames = make(map[string]string)
		} else {
			a.costumeNames = names
			log.DefaultLogger.Info().Int("count", len(names)).Msg("加载服装中文名成功")
		}

		// 加载多语言名称信息（用于搜索）
		nameInfo, err := a.apiClient.GetCostumeNameInfo(a.ctx)
		if err != nil {
			log.DefaultLogger.Warn().Err(err).Msg("加载服装名称信息失败")
			a.costumeNameInfo = make(map[string]*api.CostumeNameInfo)
		} else {
			a.costumeNameInfo = nameInfo
			log.DefaultLogger.Info().Int("count", len(nameInfo)).Msg("加载服装名称信息成功")
		}
	})
}

// lookupCostumeName 查找服装中文名称，优先使用 live2dName，其次使用 costume.
func (a *App) lookupCostumeName(live2dName, costume string) string {
	if v, ok := a.costumeNames[live2dName]; ok && v != "" {
		return v
	}
	if v, ok := a.costumeNames[costume]; ok && v != "" {
		return v
	}
	return costume
}

// sanitizeFilename 移除或替换 Windows 文件名中禁止的字符.
func sanitizeFilename(name string) string {
	replacer := strings.NewReplacer(
		"<", "＜", ">", "＞", ":", "：", `"`, "'",
		"/", "／", `\`, "＼", "|", "｜", "?", "？", "*", "＊",
	)
	name = replacer.Replace(name)
	name = strings.TrimRight(name, ". ")
	return name
}

// parsedLive2d 解析后的 Live2D 名称.
type parsedLive2d struct {
	CharaID int
	Costume string
}

// parseLive2dName 解析 Live2D 名称为角色ID和服装部分.
// 例如 "005_live_event_07_sr" → {CharaID: 5, Costume: "live_event_07_sr"}.
func parseLive2dName(live2dName string) (*parsedLive2d, bool) {
	parts := strings.Split(live2dName, "_")
	if len(parts) < 2 {
		return nil, false
	}

	foundIdx := -1
	var charaID int
	for i, p := range parts {
		id, err := strconv.Atoi(p)
		if err == nil {
			foundIdx = i
			charaID = id
			break
		}
	}
	if foundIdx == -1 || foundIdx >= len(parts)-1 {
		return nil, false
	}

	prefix := strings.Join(parts[:foundIdx], "_")
	costumePart := strings.Join(parts[foundIdx+1:], "_")
	costume := strings.Trim(strings.Join([]string{prefix, costumePart}, "_"), "_")

	return &parsedLive2d{
		CharaID: charaID,
		Costume: costume,
	}, true
}

// loadCharacterList 加载角色列表并发送到 TUI.
func (a *App) loadCharacterList() {
	charaList, err := a.apiClient.GetCharacterInfoList(a.ctx)
	if err != nil {
		log.DefaultLogger.Error().Err(err).Msg("加载角色列表失败")
		a.tuiModel.SetError(fmt.Sprintf("加载角色列表失败: %v", err))
		a.tuiModel.State = tui.StateCharaList
		return
	}

	// 过滤出有 Live2D 模型的角色
	live2dAssets, err := a.apiClient.GetLive2dAssets(a.ctx)
	if err != nil {
		log.DefaultLogger.Error().Err(err).Msg("获取Live2D资源列表失败")
		a.program.Send(tui.UpdateCharaListMsg{Characters: charaList})
		return
	}

	// 提取有 Live2D 模型的角色ID
	hasLive2d := make(map[int]bool)
	for costume := range live2dAssets {
		parts := strings.Split(costume, "_")
		for _, p := range parts {
			if id, parseErr := strconv.Atoi(p); parseErr == nil && id < 1000 {
				hasLive2d[id] = true
				break
			}
		}
	}

	// 过滤角色列表
	var filteredList []model.CharacterInfo
	for _, chara := range charaList {
		if hasLive2d[chara.ID] {
			filteredList = append(filteredList, chara)
		}
	}

	log.DefaultLogger.Info().Int("count", len(filteredList)).Msg("加载角色列表成功")
	a.program.Send(tui.UpdateCharaListMsg{Characters: filteredList})
}

// getLive2dPath 根据 Live2D 名称获取保存路径.
func (a *App) getLive2dPath(live2dName string) (string, error) {
	parsed, ok := parseLive2dName(live2dName)
	if !ok {
		log.DefaultLogger.Error().Str("live2dName", live2dName).Msg("无效的Live2D名称格式")
		return "", errors.New("无效的Live2D名称格式")
	}

	// 使用中文命名模式
	if a.tuiModel.NamingMode == config.NamingModeChinese {
		return a.getLive2dPathChinese(live2dName, parsed.CharaID, parsed.Costume)
	}

	// 原始命名模式
	return a.getLive2dPathOriginal(live2dName, parsed.CharaID)
}

// getLive2dPathChinese 使用中文命名获取保存路径.
func (a *App) getLive2dPathChinese(live2dName string, charaID int, costume string) (string, error) {
	// 加载中文名称映射
	a.loadCharacterNames()
	a.loadCostumeNames()

	// 获取角色中文名
	charaName := fmt.Sprintf("chara_%03d", charaID)
	if name, ok := a.charaNames[strconv.Itoa(charaID)]; ok && name != "" {
		charaName = sanitizeFilename(name)
	}

	// 获取服装中文名（使用完整的 live2dName 进行查找）
	costumeName := sanitizeFilename(a.lookupCostumeName(live2dName, costume))

	path := filepath.Join(config.Get().Live2dSavePath, charaName, costumeName)
	log.DefaultLogger.Info().Str("path", path).Msg("获取Live2D路径成功（中文命名）")
	return path, nil
}

// getLive2dPathOriginal 使用原始命名获取保存路径.
func (a *App) getLive2dPathOriginal(live2dName string, charaID int) (string, error) {
	charaName := a.localizedCharacterName(charaID)
	if charaName == "" {
		log.DefaultLogger.Warn().Int("charaID", charaID).Msg("获取角色信息失败，使用角色ID作为目录名")
		path := filepath.Join(
			config.Get().Live2dSavePath,
			fmt.Sprintf("chara_%03d", charaID),
			sanitizeFilename(live2dName),
		)
		log.DefaultLogger.Info().Str("path", path).Msg("获取Live2D路径成功")
		return path, nil
	}

	path := filepath.Join(config.Get().Live2dSavePath, sanitizeFilename(charaName), sanitizeFilename(live2dName))
	log.DefaultLogger.Info().Str("path", path).Msg("获取Live2D路径成功")
	return path, nil
}

// hasCompleteModel 检查当前或其他命名模式下是否已有完整模型.
func (a *App) hasCompleteModel(live2dName string, currentPath string, buildData *model.BuildData) bool {
	// 检查当前路径是否存在完整模型
	if downloader.IsModelComplete(currentPath, buildData) {
		return true
	}

	// 尝试查找其他命名模式下的路径
	parsed, ok := parseLive2dName(live2dName)
	if !ok {
		return false
	}

	savePath := config.Get().Live2dSavePath

	// 尝试中文命名路径
	a.loadCharacterNames()
	a.loadCostumeNames()
	charaName := fmt.Sprintf("chara_%03d", parsed.CharaID)
	if charaNameVal, charaNameOk := a.charaNames[strconv.Itoa(parsed.CharaID)]; charaNameOk && charaNameVal != "" {
		charaName = sanitizeFilename(charaNameVal)
	}
	// 使用完整的 live2dName 查找中文名（与 getLive2dPathChinese 一致）
	costumeName := sanitizeFilename(a.lookupCostumeName(live2dName, parsed.Costume))
	chinesePath := filepath.Join(savePath, charaName, costumeName)
	if chinesePath != currentPath {
		if downloader.IsModelComplete(chinesePath, buildData) {
			return true
		}
	}

	// 尝试原始命名路径（使用角色全名）
	fallbackName := a.localizedCharacterName(parsed.CharaID)
	originalPath := filepath.Join(savePath, sanitizeFilename(fallbackName), sanitizeFilename(live2dName))
	if fallbackName != "" && originalPath != currentPath && downloader.IsModelComplete(originalPath, buildData) {
		return true
	}

	// 尝试原始命名路径（使用角色ID）
	idPath := filepath.Join(savePath, fmt.Sprintf("chara_%03d", parsed.CharaID), sanitizeFilename(live2dName))
	if idPath != currentPath {
		if downloader.IsModelComplete(idPath, buildData) {
			return true
		}
	}

	return false
}

// downloadLive2d 下载指定的 Live2D 模型.
func (a *App) downloadLive2d(live2d *model.Live2dAsset, displayName string) error {
	log.DefaultLogger.Info().Str("live2dName", live2d.Costume).Msg("开始下载Live2D")

	server, data, err := a.apiClient.GetLive2dData(a.ctx, live2d)
	if err != nil {
		log.DefaultLogger.Error().Str("live2dName", live2d.Costume).Err(err).Msg("获取Live2D数据失败")
		return fmt.Errorf("获取Live2D数据失败: %w", err)
	}

	path, err := a.getLive2dPath(live2d.Costume)
	if err != nil {
		return err
	}

	if a.hasCompleteModel(live2d.Costume, path, data) {
		log.DefaultLogger.Info().Str("live2dName", live2d.Costume).Msg("模型已存在，跳过下载")

		// 模型已存在，更新进度到 100%
		a.updateProgressComplete(displayName, data)
		return nil
	}

	builder := downloader.NewLive2dBuilder(path, server, data, a.dl, live2d.String(), displayName)
	if constructErr := builder.Construct(); constructErr != nil {
		log.DefaultLogger.Error().Str("live2dName", live2d.Costume).Err(constructErr).Msg("构建Live2D模型失败")
		return fmt.Errorf("构建Live2D模型失败: %w", constructErr)
	}

	log.DefaultLogger.Info().Str("live2dName", live2d.Costume).Str("path", path).Msg("Live2D下载完成")
	return nil
}

// updateProgressComplete 更新进度到 100%（用于已存在模型的情况）.
func (a *App) updateProgressComplete(displayName string, data *model.BuildData) {
	totalFiles := downloader.TotalFilesForBuildData(data)

	// 添加下载项并立即设置为完成
	a.tuiModel.AddDownloadItem(displayName, totalFiles)
	a.tuiModel.UpdateProgress(displayName, totalFiles)
}

// updateCharaCostumes 更新角色服装列表.
//
//nolint:unparam // 返回值预留用于未来扩展
func (a *App) updateCharaCostumes(id int, firstName string, displayName string) bool {
	// 获取角色服装列表
	costumes, err := a.apiClient.GetCharaCostumes(a.ctx, id)
	if err != nil {
		log.DefaultLogger.Error().Int("charaID", id).Err(err).Msg("获取角色服装列表失败")
		a.tuiModel.SetError(fmt.Sprintf("获取角色服装列表失败: %v", err))
		a.tuiModel.State = tui.StateCharaList
		return true
	}

	if len(costumes) == 0 {
		log.DefaultLogger.Warn().Int("charaID", id).Msg("未找到该角色的 Live2D 模型")
		a.tuiModel.SetError("未找到该角色的 Live2D 模型")
		a.tuiModel.State = tui.StateCharaList
		return true
	}

	// 清除之前的错误消息
	a.tuiModel.ClearError()

	// 过滤出可下载的服装（验证 buildData.asset 是否存在）
	validCostumes, err := a.filterValidCostumes(costumes)
	if err != nil {
		log.DefaultLogger.Error().Int("charaID", id).Err(err).Msg("检查角色服装资源失败")
		a.tuiModel.SetError(fmt.Sprintf("检查角色服装资源失败: %v", err))
		a.tuiModel.State = tui.StateCharaList
		return true
	}

	if len(validCostumes) == 0 {
		log.DefaultLogger.Warn().Int("charaID", id).Msg("该角色没有可下载的 Live2D 模型")
		a.tuiModel.SetError("该角色没有可下载的 Live2D 模型")
		a.tuiModel.State = tui.StateCharaList
		return true
	}

	var costumeAssets []*model.Live2dAsset
	for _, live2d := range validCostumes {
		// create a copy to take address
		aCopy := live2d
		costumeAssets = append(costumeAssets, &aCopy)
	}

	// 更新列表
	a.tuiModel.CurrentCharaName = firstName
	if displayName != firstName {
		a.tuiModel.ExtraCharaName = displayName
	} else {
		a.tuiModel.ExtraCharaName = ""
	}
	log.DefaultLogger.Info().
		Str("charaName", firstName).
		Int("costumesCount", len(validCostumes)).
		Int("filteredCount", len(costumes)-len(validCostumes)).
		Msg("找到角色服装列表")

	// 加载服装名称信息
	a.loadCostumeNames()

	// 构建服装名称信息映射（用于搜索）
	costumeNameInfoMap := make(map[string]*tui.CostumeNameInfo)
	for _, asset := range costumeAssets {
		if info, ok := a.costumeNameInfo[asset.Costume]; ok {
			costumeNameInfoMap[asset.Costume] = &tui.CostumeNameInfo{
				Original: info.Original,
				Chinese:  info.Chinese,
				Japanese: info.Japanese,
			}
		} else {
			costumeNameInfoMap[asset.Costume] = &tui.CostumeNameInfo{
				Original: asset.String(),
			}
		}
	}

	// 发送列表更新（costumeNames 用于显示，costumeNameInfo 用于搜索）
	a.program.Send(tui.UpdateListMsg{
		Items:           costumeAssets,
		CostumeNames:    a.costumeNames,     // 显示用的中文名映射
		CostumeNameInfo: costumeNameInfoMap, // 搜索用的多语言信息
		CharaID:         id,
	})

	return true
}

// filterValidCostumes 过滤出可下载的服装（并发验证 buildData.asset）.
func (a *App) filterValidCostumes(costumes []model.Live2dAsset) ([]model.Live2dAsset, error) {
	type result struct {
		index int
		valid bool
		err   error
	}

	resultChan := make(chan result, len(costumes))
	sem := make(chan struct{}, 10) // 并发限制

	for i, costume := range costumes {
		go func(idx int, c model.Live2dAsset) {
			sem <- struct{}{}
			defer func() { <-sem }()

			valid, err := a.apiClient.IsLive2dAssetAvailable(a.ctx, c)
			resultChan <- result{index: idx, valid: valid, err: err}
		}(i, costume)
	}

	// 收集结果（按原始顺序）
	results := make([]result, len(costumes))
	for range costumes {
		r := <-resultChan
		results[r.index] = r
	}

	// 按原始顺序返回有效的服装
	var validCostumes []model.Live2dAsset
	for i, r := range results {
		if r.err != nil {
			return nil, fmt.Errorf("检查模型 %s 可用性失败: %w", costumes[i].Costume, r.err)
		}
		if r.valid {
			validCostumes = append(validCostumes, costumes[i])
		}
	}

	return validCostumes, nil
}

// getCharaNames 获取角色名称，如果获取失败则使用默认名称.
func (a *App) getCharaNames(id int) (string, string) {
	chara, err := a.apiClient.GetChara(a.ctx, id)
	if err != nil {
		// 如果获取角色信息失败，记录警告但继续尝试获取模型
		log.DefaultLogger.Warn().Int("charaID", id).Err(err).Msg("获取角色信息失败，尝试获取模型信息")
		defaultName := fmt.Sprintf("角色%d", id)
		return defaultName, defaultName
	}

	// 检查角色信息格式
	characterNames, ok := chara["characterName"].([]any)
	if !ok || len(characterNames) == 0 {
		log.DefaultLogger.Error().Int("charaID", id).Msg("无效的角色名字格式")
		defaultName := fmt.Sprintf("角色%d", id)
		return defaultName, defaultName
	}

	firstName, _ := characterNames[0].(string)
	firstName = strings.TrimSpace(firstName)

	displayName := a.localizedCharacterName(id)
	if firstName == "" {
		firstName = displayName
	}
	if displayName == "" {
		displayName = firstName
	}
	if displayName == "" {
		defaultName := fmt.Sprintf("角色%d", id)
		return defaultName, defaultName
	}

	return firstName, displayName
}

// downloadModel 下载单个模型.
func (a *App) downloadModel(
	asset *model.Live2dAsset,
	displayName string,
	errChan chan error,
	completed map[string]bool,
	progressUpdated chan struct{},
) {
	// 使用传入的翻译名
	name := displayName
	if name == "" && asset != nil {
		name = asset.String()
	}

	if err := a.downloadLive2d(asset, displayName); err != nil {
		if err.Error() == ErrDownloadCancelled {
			errChan <- err
			return
		}
		log.DefaultLogger.Error().Str("model", name).Err(err).Msg("下载失败")
		a.tuiModel.SendError(name, err)
	} else {
		completed[name] = true
		// 只有成功才更新总体进度
		a.tuiModel.UpdateTotalProgress()
	}
	// 通知进度已更新
	select {
	case progressUpdated <- struct{}{}:
	default:
	}
}

// handleBatchDownload 处理批量下载请求.
func (a *App) handleBatchDownload(selectedItems []*tui.SelectedItem) bool {
	if len(selectedItems) == 0 {
		return true
	}

	log.DefaultLogger.Info().Int("selectedCount", len(selectedItems)).Msg("开始批量下载Live2D")

	// 设置总体进度
	a.tuiModel.SetTotalModels(len(selectedItems))

	errChan := make(chan error, 1)
	completed := make(map[string]bool)
	modelSem := make(chan struct{}, config.Get().MaxConcurrentModels)
	progressUpdated := make(chan struct{}, 1) // 用于通知进度已更新

	for _, item := range selectedItems {
		select {
		case <-a.ctx.Done():
			a.handleCancelledDownloads(selectedItems, completed)
			return false
		case err := <-errChan:
			if err.Error() == ErrDownloadCancelled {
				a.handleCancelledDownloads(selectedItems, completed)
				return false
			}
			log.DefaultLogger.Error().Err(err).Msg("下载失败")
			continue
		default:
			modelSem <- struct{}{}
			go func(item *tui.SelectedItem) {
				defer func() { <-modelSem }()
				a.downloadModel(item.Asset, item.DisplayName, errChan, completed, progressUpdated)
			}(item)
		}
	}

	for range cap(modelSem) {
		modelSem <- struct{}{}
	}
	log.DefaultLogger.Info().Msg("批量下载完成")
	return true
}

// handleCancelledDownloads 处理已取消的下载.
func (a *App) handleCancelledDownloads(selectedItems []*tui.SelectedItem, completed map[string]bool) {
	for _, item := range selectedItems {
		// 使用翻译名
		name := item.DisplayName
		if name == "" && item.Asset != nil {
			name = item.Asset.String()
		}

		if !completed[name] {
			log.DefaultLogger.Error().Str("model", name).Msg("下载已取消")
		}
	}
}

// Run 运行应用程序.
func (a *App) Run() {
	a.initialize()
	log.DefaultLogger.Info().Msg("程序启动")
	defer a.cancel()

	// 启动 TUI
	go func() {
		if _, err := a.program.Run(); err != nil {
			log.DefaultLogger.Error().Err(err).Msg("运行程序时出错")
			os.Exit(1)
		}
	}()

	// 加载角色列表
	go a.loadCharacterList()

	// 处理角色选择和下载
	for {
		select {
		case <-a.ctx.Done():
			log.DefaultLogger.Info().Msg("程序正常退出")
			return
		case <-a.tuiModel.GetCancelChan():
			a.cancel()
			return
		case charaID := <-a.tuiModel.GetCharaSelectChan():
			firstName, displayName := a.getCharaNames(charaID)
			a.updateCharaCostumes(charaID, firstName, displayName)
		case selectedItems := <-a.tuiModel.GetSelectChan():
			if !a.handleBatchDownload(selectedItems) {
				return
			}
		}
	}
}

// main 函数是程序的入口点.
func main() {
	app := NewApp()
	app.Run()
}
