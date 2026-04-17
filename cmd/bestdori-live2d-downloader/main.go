// Package main 是 Bestdori Live2D 下载器的主程序包
// 该程序用于从 Bestdori 网站下载 Live2D 模型，支持角色搜索和直接下载
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/A-kirami/bestdori-live2d-downloader/pkg/api"
	"github.com/A-kirami/bestdori-live2d-downloader/pkg/config"
	"github.com/A-kirami/bestdori-live2d-downloader/pkg/downloader"
	"github.com/A-kirami/bestdori-live2d-downloader/pkg/log"
	"github.com/A-kirami/bestdori-live2d-downloader/pkg/matcher"
	"github.com/A-kirami/bestdori-live2d-downloader/pkg/model"
	"github.com/A-kirami/bestdori-live2d-downloader/pkg/tui"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	// SplitPartsCount 表示字符串分割后的预期部分数量.
	SplitPartsCount = 2

	// StateInput 表示输入状态.
	StateInput = "input"

	// ErrDownloadCancelled 表示下载已取消的错误.
	ErrDownloadCancelled = "下载已取消"
)

// SuggestionError 表示建议类型的错误.
type SuggestionError struct {
	Message   string
	BestMatch string
}

func (e *SuggestionError) Error() string {
	return e.Message
}

// IsSuggestionError 检查错误是否为建议类型.
func IsSuggestionError(err error) bool {
	suggestionError := &SuggestionError{}
	ok := errors.As(err, &suggestionError)
	return ok
}

// App 表示应用程序的主要结构.
type App struct {
	ctx       context.Context
	cancel    context.CancelFunc
	apiClient *api.Client
	dl        *downloader.Downloader
	tuiModel  *tui.Model
	program   *tea.Program
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

// getLive2dPath 根据 Live2D 名称获取保存路径.
func (a *App) getLive2dPath(live2dName string) (string, error) {
	parts := strings.Split(live2dName, "_")
	if len(parts) == 0 {
		log.DefaultLogger.Error().Str("live2dName", live2dName).Msg("无效的Live2D名称格式")
		return "", errors.New("无效的Live2D名称格式")
	}

	// 找到第一个可解析的角色ID位置
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
	if foundIdx == -1 {
		log.DefaultLogger.Error().Str("live2dName", live2dName).Msg("未找到可解析为数字的角色ID")
		return "", errors.New("无效的角色ID: 未找到可解析为数字的部分")
	}

	// 角色ID 后必须还有服装部分
	if foundIdx >= len(parts)-1 {
		log.DefaultLogger.Error().Str("live2dName", live2dName).Msg("无效的Live2D名称格式: 缺少服装部分")
		return "", errors.New("无效的Live2D名称格式: 缺少服装部分")
	}

	prefix := strings.Join(parts[:foundIdx], "_")        // 可能为空
	costumePart := strings.Join(parts[foundIdx+1:], "_") // 服装部分
	costume := strings.Trim(strings.Join([]string{prefix, costumePart}, "_"), "_")

	// 后续逻辑仅使用 charaID 和 costume
	chara, err := a.apiClient.GetChara(a.ctx, charaID)
	if err != nil {
		log.DefaultLogger.Warn().Int("charaID", charaID).Err(err).Msg("获取角色信息失败，使用角色ID作为目录名")
		path := filepath.Join(config.Get().Live2dSavePath, fmt.Sprintf("chara_%03d", charaID), costumePart)
		log.DefaultLogger.Info().Str("path", path).Msg("获取Live2D路径成功")
		return path, nil
	}

	// 如果成功获取角色信息，使用角色名作为目录名
	firstName, ok := chara["firstName"].([]any)[1].(string)
	if !ok {
		// 如果无法获取角色名，使用角色ID作为目录名
		log.DefaultLogger.Warn().Int("charaID", charaID).Msg("无效的角色名字格式，使用角色ID作为目录名")
		path := filepath.Join(config.Get().Live2dSavePath, fmt.Sprintf("chara_%03d", charaID), costumePart)
		log.DefaultLogger.Info().Str("path", path).Msg("获取Live2D路径成功")
		return path, nil
	}

	path := filepath.Join(config.Get().Live2dSavePath, strings.ToLower(firstName), costume)
	log.DefaultLogger.Info().Str("path", path).Msg("获取Live2D路径成功")
	return path, nil
}

// downloadLive2d 下载指定的 Live2D 模型.
func (a *App) downloadLive2d(live2d *model.Live2dAsset) error {
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

	builder := downloader.NewLive2dBuilder(path, server, data, a.dl, live2d.String())
	if constructErr := builder.Construct(); constructErr != nil {
		log.DefaultLogger.Error().Str("live2dName", live2d.Costume).Err(constructErr).Msg("构建Live2D模型失败")
		return fmt.Errorf("构建Live2D模型失败: %w", constructErr)
	}

	log.DefaultLogger.Info().Str("live2dName", live2d.Costume).Str("path", path).Msg("Live2D下载完成")
	return nil
}

// findChara 根据名称搜索角色.
func (a *App) findChara(name string) (*model.MatchChara, error) {
	log.DefaultLogger.Info().Str("name", name).Msg("开始搜索角色")

	characterRoster, err := a.apiClient.GetCharaRoster(a.ctx)
	if err != nil {
		log.DefaultLogger.Error().Str("name", name).Err(err).Msg("获取角色列表失败")
		return nil, fmt.Errorf("获取角色列表失败: %w", err)
	}

	candidates := make(map[string][]string)
	for charaID, info := range characterRoster {
		charaIDNum, parseErr := strconv.Atoi(charaID)
		if parseErr != nil || charaIDNum > 1000 {
			continue
		}

		charaInfo, ok := info.(map[string]any)
		if !ok {
			continue
		}
		characterNames, ok := charaInfo["characterName"].([]any)
		if !ok {
			continue
		}
		names := make([]string, len(characterNames))
		for i := range characterNames {
			characterName, nameOk := characterNames[i].(string)
			if !nameOk {
				continue
			}
			names[i] = characterName
		}
		candidates[charaID] = names
	}

	bestID, bestMatch, maxSimilarity := matcher.FindBestMatch(name, candidates)
	// 设置相似度阈值，用于判断是否为高置信度匹配
	const similarityThreshold = 0.6

	if maxSimilarity < similarityThreshold {
		log.DefaultLogger.Warn().
			Str("name", name).
			Str("bestMatch", bestMatch).
			Float64("similarity", maxSimilarity).
			Float64("threshold", similarityThreshold).
			Msg("未找到足够相似的角色，但提供最佳建议")
		return nil, &SuggestionError{
			Message:   fmt.Sprintf("未找到符合此名称的角色，你要找的是「%s」吗？", bestMatch),
			BestMatch: bestMatch,
		}
	}

	id, _ := strconv.Atoi(bestID)
	log.DefaultLogger.Info().
		Str("name", name).
		Str("bestMatch", bestMatch).
		Float64("similarity", maxSimilarity).
		Float64("threshold", similarityThreshold).
		Msg("找到匹配的角色")
	return &model.MatchChara{
		ID:    id,
		Name:  bestMatch,
		Names: candidates[bestID],
	}, nil
}

// updateCharaCostumes 更新角色服装列表.
func (a *App) updateCharaCostumes(id int, firstName string, displayName string) bool {
	// 获取角色服装列表
	costumes, err := a.apiClient.GetCharaCostumes(a.ctx, id)
	if err != nil {
		log.DefaultLogger.Error().Int("charaID", id).Err(err).Msg("获取角色服装列表失败")
		a.tuiModel.SetError(fmt.Sprintf("获取角色服装列表失败: %v", err))
		a.tuiModel.State = StateInput
		return true
	}

	if len(costumes) == 0 {
		log.DefaultLogger.Warn().Int("charaID", id).Msg("未找到该角色的 Live2D 模型")
		a.tuiModel.SetError("未找到该角色的 Live2D 模型")
		a.tuiModel.State = StateInput
		return true
	}

	// 清除之前的错误消息
	a.tuiModel.ClearError()

	var costumeAssets []*model.Live2dAsset
	for _, live2d := range costumes {
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
		Int("costumesCount", len(costumes)).
		Msg("找到角色服装列表")
	a.program.Send(tui.UpdateListMsg{Items: costumeAssets})

	return true
}

// handleCharaIDSearch 处理角色编号搜索请求.
func (a *App) handleCharaIDSearch(charaID string) bool {
	id, err := strconv.Atoi(charaID)
	if err != nil {
		log.DefaultLogger.Error().Str("charaID", charaID).Err(err).Msg("无效的角色编号")
		a.tuiModel.SetError(fmt.Sprintf("无效的角色编号: %s", charaID))
		a.tuiModel.State = StateInput
		return true
	}

	firstName, displayName := a.getCharaNames(id)
	return a.updateCharaCostumes(id, firstName, displayName)
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
	if !ok || len(characterNames) < 4 {
		log.DefaultLogger.Error().Int("charaID", id).Msg("无效的角色名字格式")
		defaultName := fmt.Sprintf("角色%d", id)
		return defaultName, defaultName
	}

	// 检查每个元素是否为字符串
	firstName, ok := characterNames[0].(string)
	if !ok {
		log.DefaultLogger.Error().Int("charaID", id).Msg("角色名字格式错误")
		defaultName := fmt.Sprintf("角色%d", id)
		return defaultName, defaultName
	}

	displayName, ok := characterNames[3].(string)
	if !ok || displayName == "" {
		displayName = firstName
	}

	return firstName, displayName
}

// handleCharaSearch 处理角色搜索请求.
func (a *App) handleCharaSearch(input string) bool {
	matchChara, err := a.findChara(input)
	if err != nil {
		// 检查是否为建议错误（相似度不够高的情况）
		if IsSuggestionError(err) {
			log.DefaultLogger.Warn().Str("input", input).Err(err).Msg("提供角色建议")
			a.tuiModel.SetError(err.Error())
			a.tuiModel.State = StateInput
			return true
		}

		log.DefaultLogger.Error().Str("input", input).Err(err).Msg("搜索角色失败")
		a.tuiModel.SetError(fmt.Sprintf("搜索角色失败: %v", err))
		a.tuiModel.State = StateInput
		return true
	}
	if matchChara == nil {
		log.DefaultLogger.Warn().Str("input", input).Msg("未找到角色")
		a.tuiModel.SetError(fmt.Sprintf("未找到角色: %s", input))
		a.tuiModel.State = StateInput
		return true
	}

	// 使用与 main.go 相同的名称逻辑
	displayName := matchChara.Names[3]
	if displayName == "" {
		displayName = matchChara.Names[0]
	}

	return a.updateCharaCostumes(matchChara.ID, matchChara.Name, displayName)
}

func (a *App) resolveDirectDownloadAssets(modelNames []string) ([]*model.Live2dAsset, []string, error) {
	assets := make([]*model.Live2dAsset, 0, len(modelNames))
	invalidModels := make([]string, 0)

	for _, name := range modelNames {
		asset, exists, err := a.apiClient.GetLive2dAsset(a.ctx, name)
		if err != nil {
			return nil, nil, fmt.Errorf("验证模型失败: %w", err)
		}
		if !exists {
			invalidModels = append(invalidModels, name)
			continue
		}
		assets = append(assets, asset)
	}

	return assets, invalidModels, nil
}

func (a *App) shouldHandleAsDirectDownload(input string) (bool, error) {
	if input == "" {
		return false, nil
	}

	_, exists, err := a.apiClient.GetLive2dAsset(a.ctx, input)
	if err != nil {
		return false, fmt.Errorf("验证模型失败: %w", err)
	}

	return exists, nil
}

// handleDirectDownload 处理直接下载请求.
func (a *App) handleDirectDownload(input string) bool {
	log.DefaultLogger.Info().Str("input", input).Msg("开始直接下载Live2D")

	// 分割输入字符串，支持空格、中文逗号和英文逗号作为分隔符
	inputs := strings.FieldsFunc(input, func(r rune) bool {
		return r == ' ' || r == ',' || r == '，'
	})

	// 移除每个模型名可能存在的 _rip 后缀
	modelNames := make([]string, 0, len(inputs))
	for _, name := range inputs {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		modelNames = append(modelNames, strings.TrimSuffix(name, "_rip"))
	}

	if len(modelNames) == 0 {
		log.DefaultLogger.Error().Str("input", input).Msg("没有有效的模型名称")
		a.tuiModel.SetError("没有有效的模型名称")
		a.tuiModel.State = StateInput
		return true
	}

	assets, invalidModels, err := a.resolveDirectDownloadAssets(modelNames)
	if err != nil {
		log.DefaultLogger.Error().Strs("models", modelNames).Err(err).Msg("验证模型失败")
		a.tuiModel.SetError(err.Error())
		a.tuiModel.State = StateInput
		return true
	}

	// 如果有无效的模型，显示错误信息
	if len(invalidModels) > 0 {
		errorMsg := fmt.Sprintf("以下模型不存在: %s", strings.Join(invalidModels, ", "))
		log.DefaultLogger.Error().Strs("invalidModels", invalidModels).Msg("发现无效的模型名称")
		a.tuiModel.SetError(errorMsg)
		a.tuiModel.State = StateInput
		return true
	}

	a.tuiModel.State = "downloading"
	a.tuiModel.DownloadList.Title = "下载进度"

	// 使用批量下载功能处理多个模型
	return a.handleBatchDownload(assets)
}

// handleDownload 处理下载请求.
func (a *App) handleDownload(input string) bool {
	// 检查是否为纯数字
	if _, err := strconv.Atoi(input); err == nil {
		// 如果是纯数字，直接搜索该编号的角色
		return a.handleCharaIDSearch(input)
	}

	// 优先按完整模型名称处理，再回退到角色搜索
	direct, err := a.shouldHandleAsDirectDownload(input)
	if err != nil {
		log.DefaultLogger.Error().Str("input", input).Err(err).Msg("验证模型失败")
		a.tuiModel.SetError(err.Error())
		a.tuiModel.State = StateInput
		return true
	}
	if direct {
		return a.handleDirectDownload(input)
	}

	// 如果不是模型名称，则尝试角色搜索
	return a.handleCharaSearch(input)
}

// downloadModel 下载单个模型.
func (a *App) downloadModel(
	asset *model.Live2dAsset,
	errChan chan error,
	completed map[string]bool,
	progressUpdated chan struct{},
) {
	name := ""
	if asset != nil {
		name = asset.String()
	}

	if err := a.downloadLive2d(asset); err != nil {
		if err.Error() == ErrDownloadCancelled {
			errChan <- err
			return
		}
		log.DefaultLogger.Error().Str("model", name).Err(err).Msg("下载失败")
	} else {
		completed[name] = true
	}
	// 无论成功还是失败，都更新总体进度
	a.tuiModel.UpdateTotalProgress()
	// 通知进度已更新
	select {
	case progressUpdated <- struct{}{}:
	default:
	}
}

// handleBatchDownload 处理批量下载请求.
func (a *App) handleBatchDownload(selectedItems []*model.Live2dAsset) bool {
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

	for _, asset := range selectedItems {
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
			go func(asset *model.Live2dAsset) {
				defer func() { <-modelSem }()
				a.downloadModel(asset, errChan, completed, progressUpdated)
			}(asset)
		}
	}

	for range cap(modelSem) {
		modelSem <- struct{}{}
	}
	log.DefaultLogger.Info().Msg("批量下载完成")
	return true
}

// handleCancelledDownloads 处理已取消的下载.
func (a *App) handleCancelledDownloads(selectedItems []*model.Live2dAsset, completed map[string]bool) {
	for _, asset := range selectedItems {
		name := ""
		if asset != nil {
			name = asset.String()
		}
		if !completed[name] {
			log.DefaultLogger.Error().Str("model", name).Msg("下载已取消")
			// 注意：总体进度已经在downloadModel中更新，这里不需要重复更新
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

	// 处理用户输入和下载
	for {
		select {
		case <-a.ctx.Done():
			log.DefaultLogger.Info().Msg("程序正常退出")
			return
		case <-a.tuiModel.GetCancelChan():
			a.cancel()
			return
		case input := <-a.tuiModel.GetSearchChan():
			if input == "q" {
				a.cancel()
				return
			}

			if !a.handleDownload(input) {
				return
			}
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
