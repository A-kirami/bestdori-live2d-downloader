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
	"github.com/A-kirami/bestdori-live2d-downloader/pkg/model"
	"github.com/A-kirami/bestdori-live2d-downloader/pkg/tui"
	"github.com/A-kirami/bestdori-live2d-downloader/pkg/utils"

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
	parts := strings.SplitN(live2dName, "_", SplitPartsCount)
	if len(parts) != SplitPartsCount {
		log.DefaultLogger.Error().Str("live2dName", live2dName).Msg("无效的Live2D名称格式")
		return "", errors.New("无效的Live2D名称格式")
	}

	charaID, err := strconv.Atoi(parts[0])
	if err != nil {
		log.DefaultLogger.Error().Str("live2dName", live2dName).Err(err).Msg("无效的角色ID")
		return "", fmt.Errorf("无效的角色ID: %w", err)
	}

	// 尝试获取角色信息
	chara, err := a.apiClient.GetChara(a.ctx, charaID)
	if err != nil {
		// 如果获取角色信息失败，使用角色ID作为目录名
		log.DefaultLogger.Warn().Int("charaID", charaID).Err(err).Msg("获取角色信息失败，使用角色ID作为目录名")
		path := filepath.Join(config.Get().Live2dSavePath, fmt.Sprintf("chara_%03d", charaID), parts[1])
		log.DefaultLogger.Info().Str("path", path).Msg("获取Live2D路径成功")
		return path, nil
	}

	// 如果成功获取角色信息，使用角色名作为目录名
	firstName, ok := chara["firstName"].([]any)[1].(string)
	if !ok {
		// 如果无法获取角色名，使用角色ID作为目录名
		log.DefaultLogger.Warn().Int("charaID", charaID).Msg("无效的角色名字格式，使用角色ID作为目录名")
		path := filepath.Join(config.Get().Live2dSavePath, fmt.Sprintf("chara_%03d", charaID), parts[1])
		log.DefaultLogger.Info().Str("path", path).Msg("获取Live2D路径成功")
		return path, nil
	}

	path := filepath.Join(config.Get().Live2dSavePath, strings.ToLower(firstName), parts[1])
	log.DefaultLogger.Info().Str("path", path).Msg("获取Live2D路径成功")
	return path, nil
}

// downloadLive2d 下载指定的 Live2D 模型.
func (a *App) downloadLive2d(live2dName string) error {
	log.DefaultLogger.Info().Str("live2dName", live2dName).Msg("开始下载Live2D")

	data, err := a.apiClient.GetLive2dData(a.ctx, live2dName)
	if err != nil {
		log.DefaultLogger.Error().Str("live2dName", live2dName).Err(err).Msg("获取Live2D数据失败")
		return fmt.Errorf("获取Live2D数据失败: %w", err)
	}

	path, err := a.getLive2dPath(live2dName)
	if err != nil {
		return err
	}

	builder := downloader.NewLive2dBuilder(path, data, a.dl, live2dName)
	if constructErr := builder.Construct(); constructErr != nil {
		log.DefaultLogger.Error().Str("live2dName", live2dName).Err(constructErr).Msg("构建Live2D模型失败")
		return fmt.Errorf("构建Live2D模型失败: %w", constructErr)
	}

	log.DefaultLogger.Info().Str("live2dName", live2dName).Str("path", path).Msg("Live2D下载完成")
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

	bestID, bestMatch, maxSimilarity := utils.FindBestMatch(name, candidates)
	if maxSimilarity == 0 {
		log.DefaultLogger.Warn().Str("name", name).Msg("未找到匹配的角色")
		return nil, errors.New("未找到匹配的角色")
	}

	id, _ := strconv.Atoi(bestID)
	log.DefaultLogger.Info().
		Str("name", name).
		Str("bestMatch", bestMatch).
		Float64("similarity", maxSimilarity).
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
	a.program.Send(tui.UpdateListMsg{Items: costumes})

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

	// 获取角色信息
	chara, err := a.apiClient.GetChara(a.ctx, id)
	if err != nil {
		log.DefaultLogger.Error().Int("charaID", id).Err(err).Msg("获取角色信息失败")
		a.tuiModel.SetError(fmt.Sprintf("获取角色信息失败: %v", err))
		a.tuiModel.State = StateInput
		return true
	}

	// 检查角色信息格式
	characterNames, ok := chara["characterName"].([]any)
	if !ok {
		log.DefaultLogger.Error().Int("charaID", id).Msg("无效的角色名字格式")
		a.tuiModel.SetError("无效的角色名字格式")
		a.tuiModel.State = StateInput
		return true
	}

	// 确保数组长度足够
	if len(characterNames) < 4 {
		log.DefaultLogger.Error().Int("charaID", id).Msg("角色名字数组长度不足")
		a.tuiModel.SetError("无效的角色名字格式")
		a.tuiModel.State = StateInput
		return true
	}

	// 检查每个元素是否为字符串
	firstName, ok := characterNames[0].(string)
	if !ok {
		log.DefaultLogger.Error().Int("charaID", id).Msg("角色名字格式错误")
		a.tuiModel.SetError("无效的角色名字格式")
		a.tuiModel.State = StateInput
		return true
	}

	displayName, ok := characterNames[3].(string)
	if !ok || displayName == "" {
		displayName = firstName
	}

	return a.updateCharaCostumes(id, firstName, displayName)
}

// handleCharaSearch 处理角色搜索请求.
func (a *App) handleCharaSearch(input string) bool {
	matchChara, err := a.findChara(input)
	if err != nil {
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

// handleDirectDownload 处理直接下载请求.
func (a *App) handleDirectDownload(input string) bool {
	log.DefaultLogger.Info().Str("input", input).Msg("开始直接下载Live2D")

	// 如果输入已经包含 _rip 后缀，则移除它
	if strings.HasSuffix(input, "_rip") {
		input = strings.TrimSuffix(input, "_rip")
	}

	// 初始化下载列表
	a.tuiModel.AddDownloadItem(input, 1) // 初始总数为1，后续会更新
	a.tuiModel.State = "downloading"
	a.tuiModel.DownloadList.Title = "下载进度"

	if downloadErr := a.downloadLive2d(input); downloadErr != nil {
		if downloadErr.Error() == ErrDownloadCancelled {
			log.DefaultLogger.Info().Str("input", input).Msg("下载已取消")
			return false
		}
		log.DefaultLogger.Error().Str("input", input).Err(downloadErr).Msg("下载失败")
		a.tuiModel.SetError(fmt.Sprintf("下载失败: %v", downloadErr))
		a.tuiModel.State = StateInput // 重置状态到输入模式
		return true
	}
	return true
}

// handleDownload 处理下载请求.
func (a *App) handleDownload(input string) bool {
	// 检查是否为纯数字
	if _, err := strconv.Atoi(input); err == nil {
		// 如果是纯数字，直接搜索该编号的角色
		return a.handleCharaIDSearch(input)
	}

	// 先尝试作为 Live2D 模型名称处理
	parts := strings.SplitN(input, "_", SplitPartsCount)
	if len(parts) >= 2 {
		if _, err := strconv.Atoi(parts[0]); err == nil {
			return a.handleDirectDownload(input)
		}
	}

	// 如果不是模型名称，则尝试角色搜索
	return a.handleCharaSearch(input)
}

// downloadModel 下载单个模型.
func (a *App) downloadModel(costume string, errChan chan error, completed map[string]bool) {
	if err := a.downloadLive2d(costume); err != nil {
		if err.Error() == ErrDownloadCancelled {
			errChan <- err
			return
		}
		log.DefaultLogger.Error().Str("model", costume).Err(err).Msg("下载失败")
	} else {
		completed[costume] = true
	}
}

// handleBatchDownload 处理批量下载请求.
func (a *App) handleBatchDownload(selectedItems []string) bool {
	if len(selectedItems) == 0 {
		return true
	}

	log.DefaultLogger.Info().Int("selectedCount", len(selectedItems)).Msg("开始批量下载Live2D")

	errChan := make(chan error, 1)
	completed := make(map[string]bool)
	modelSem := make(chan struct{}, config.Get().MaxConcurrentModels)

	for _, costume := range selectedItems {
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
			go func(costume string) {
				defer func() { <-modelSem }()
				a.downloadModel(costume, errChan, completed)
			}(costume)
		}
	}

	for range cap(modelSem) {
		modelSem <- struct{}{}
	}
	log.DefaultLogger.Info().Msg("批量下载完成")
	return true
}

// handleCancelledDownloads 处理已取消的下载.
func (a *App) handleCancelledDownloads(selectedItems []string, completed map[string]bool) {
	for _, item := range selectedItems {
		if !completed[item] {
			log.DefaultLogger.Error().Str("model", item).Msg("下载已取消")
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
