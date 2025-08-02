// Package downloader 提供了下载和构建 Live2D 模型的功能
// 包括文件下载、并发控制、进度显示等功能
package downloader

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/A-kirami/bestdori-live2d-downloader/pkg/api"
	"github.com/A-kirami/bestdori-live2d-downloader/pkg/config"
	"github.com/A-kirami/bestdori-live2d-downloader/pkg/log"
	"github.com/A-kirami/bestdori-live2d-downloader/pkg/model"
	"github.com/A-kirami/bestdori-live2d-downloader/pkg/tui"

	tea "github.com/charmbracelet/bubbletea"
)

// MotionFile 表示动作文件的类型.
type MotionFile = model.MotionFile

// ExpressionFile 表示表情文件的类型.
type ExpressionFile = model.ExpressionFile

// downloadTask 表示下载任务.
type downloadTask struct {
	bundleFile    model.BundleFile    // 要下载的资源包文件信息
	filePath      string              // 保存路径
	allowNotFound bool                // 是否允许文件不存在
	result        chan downloadResult // 结果通道
}

// downloadResult 表示下载结果.
type downloadResult struct {
	relPath string // 相对路径
	err     error  // 错误信息
}

// Downloader 表示下载器
// 负责处理文件下载、并发控制和进度显示.
type Downloader struct {
	apiClient  *api.Client   // API 客户端
	savePath   string        // 保存路径
	TuiModel   *tui.Model    // TUI 模型
	program    *tea.Program  // TUI 程序
	modelSem   chan struct{} // 模型并发控制信号量
	httpClient *http.Client  // HTTP 客户端
}

// NewDownloader 创建新的下载器实例
// 参数:
//   - apiClient: API 客户端实例
//   - tuiModel: TUI 模型实例
//   - program: TUI 程序实例
//
// 返回:
//   - *Downloader: 新的下载器实例
func NewDownloader(apiClient *api.Client, tuiModel *tui.Model, program *tea.Program) *Downloader {
	cfg := config.Get()
	return &Downloader{
		apiClient: apiClient,
		savePath:  cfg.Live2dSavePath,
		TuiModel:  tuiModel,
		program:   program,
		modelSem:  make(chan struct{}, cfg.MaxConcurrentModels),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// createDownloadRequest 创建下载请求
// 参数:
//   - ctx: 上下文
//   - bundleFile: 资源包文件信息
//
// 返回:
//   - *http.Request: HTTP请求
//   - error: 错误信息
func (d *Downloader) createDownloadRequest(ctx context.Context, bundleFile model.BundleFile) (*http.Request, error) {
	url := fmt.Sprintf("%s/%s_rip/%s", config.Get().BaseAssetsURL, bundleFile.BundleName, bundleFile.FileName)
	log.DefaultLogger.Info().Str("url", url).Msg("开始下载文件")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		log.DefaultLogger.Error().Str("url", url).Err(err).Msg("创建请求失败")
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	return req, nil
}

// validateResponse 验证HTTP响应
// 参数:
//   - resp: HTTP响应
//   - url: 请求URL
//   - allowNotFound: 是否允许文件不存在
//
// 返回:
//   - error: 错误信息
func (d *Downloader) validateResponse(resp *http.Response, url string, allowNotFound bool) error {
	if resp.StatusCode != http.StatusOK {
		// 如果允许文件不存在，404错误被视为正常情况
		if allowNotFound && resp.StatusCode == http.StatusNotFound {
			log.DefaultLogger.Info().Str("url", url).Msg("文件不存在，跳过下载")
			return nil
		}
		log.DefaultLogger.Error().Str("url", url).Int("statusCode", resp.StatusCode).Msg("下载文件HTTP错误")
		return fmt.Errorf("下载文件HTTP错误: %d", resp.StatusCode)
	}

	// 检查Content-Type是否为HTML，如果是则说明是错误页面
	contentType := resp.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "text/html") {
		log.DefaultLogger.Error().Str("url", url).Str("contentType", contentType).Msg("文件不存在或无法访问")
		return errors.New("文件不存在或无法访问")
	}

	return nil
}

// createFileAndDirectory 创建文件和目录
// 参数:
//   - filePath: 文件路径
//
// 返回:
//   - *os.File: 文件句柄
//   - error: 错误信息
func (d *Downloader) createFileAndDirectory(filePath string) (*os.File, error) {
	if mkdirErr := os.MkdirAll(filepath.Dir(filePath), 0750); mkdirErr != nil {
		log.DefaultLogger.Error().Str("filePath", filePath).Err(mkdirErr).Msg("创建目录失败")
		return nil, fmt.Errorf("创建目录失败: %w", mkdirErr)
	}

	file, err := os.Create(filePath)
	if err != nil {
		log.DefaultLogger.Error().Str("filePath", filePath).Err(err).Msg("创建文件失败")
		return nil, fmt.Errorf("创建文件失败: %w", err)
	}

	return file, nil
}

// writeFileContent 写入文件内容
// 参数:
//   - file: 文件句柄
//   - resp: HTTP响应
//   - filePath: 文件路径
//
// 返回:
//   - error: 错误信息
func (d *Downloader) writeFileContent(file *os.File, resp *http.Response, filePath string) error {
	_, err := io.Copy(file, resp.Body)
	if err != nil {
		// 判断是否为 context 超时或取消
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			log.DefaultLogger.Error().Str("filePath", filePath).Err(err).Msg("下载超时或被取消")
			return fmt.Errorf("下载超时或被取消: %w", err)
		}
		log.DefaultLogger.Error().Str("filePath", filePath).Err(err).Msg("写入文件失败")
		return fmt.Errorf("写入文件失败: %w", err)
	}
	return nil
}

// DownloadBundleFile 下载资源包文件
// 参数:
//   - ctx: 上下文
//   - bundleFile: 资源包文件信息
//   - filePath: 保存路径
//   - allowNotFound: 是否允许文件不存在（404错误时视为正常情况）
//
// 返回:
//   - error: 错误信息
func (d *Downloader) DownloadBundleFile(
	ctx context.Context,
	bundleFile model.BundleFile,
	filePath string,
	allowNotFound bool,
) error {
	select {
	case <-ctx.Done():
		log.DefaultLogger.Info().Str("filePath", filePath).Msg("下载已取消")
		return errors.New("下载已取消")
	default:
	}

	// 创建请求
	req, err := d.createDownloadRequest(ctx, bundleFile)
	if err != nil {
		return err
	}

	// 执行请求
	resp, err := d.httpClient.Do(req)
	if err != nil {
		log.DefaultLogger.Error().Str("url", req.URL.String()).Err(err).Msg("下载文件失败")
		return fmt.Errorf("下载文件失败: %w", err)
	}
	defer resp.Body.Close()

	// 验证响应
	if validateErr := d.validateResponse(resp, req.URL.String(), allowNotFound); validateErr != nil {
		return validateErr
	}

	// 如果允许文件不存在且文件不存在，直接返回
	if allowNotFound && resp.StatusCode == http.StatusNotFound {
		return nil
	}

	// 创建文件和目录
	file, createErr := d.createFileAndDirectory(filePath)
	if createErr != nil {
		return createErr
	}
	defer file.Close()

	// 写入文件内容
	if writeErr := d.writeFileContent(file, resp, filePath); writeErr != nil {
		return writeErr
	}

	log.DefaultLogger.Info().Str("filePath", filePath).Msg("文件下载完成")
	return nil
}

// Live2dBuilder 表示 Live2D 构建器
// 负责构建完整的 Live2D 模型，包括下载所有必要文件.
type Live2dBuilder struct {
	path       string             // 模型保存路径
	data       *model.BuildData   // 构建数据
	model      *model.Live2dModel // Live2D 模型
	dataPath   string             // 数据文件路径
	downloader *Downloader        // 下载器实例
	ModelName  string             // 模型名称
}

// NewLive2dBuilder 创建新的 Live2D 构建器实例
// 参数:
//   - path: 模型保存路径
//   - buildData: 构建数据
//   - downloader: 下载器实例
//   - modelName: 模型名称
//
// 返回:
//   - *Live2dBuilder: 新的 Live2D 构建器实例
func NewLive2dBuilder(
	path string,
	buildData *model.BuildData,
	downloader *Downloader,
	modelName string,
) *Live2dBuilder {
	return &Live2dBuilder{
		path:       path,
		data:       buildData,
		model:      &model.Live2dModel{Motions: make(map[string][]model.MotionFile)},
		dataPath:   filepath.Join(path, "data"),
		downloader: downloader,
		ModelName:  modelName,
	}
}

// ProcessFile 处理单个文件
// 参数:
//   - ctx: 上下文
//   - bundleFile: 资源包文件信息
//   - filePath: 保存路径
//   - allowNotFound: 是否允许文件不存在
//
// 返回:
//   - string: 相对路径
//   - error: 错误信息
func (b *Live2dBuilder) ProcessFile(
	ctx context.Context,
	bundleFile model.BundleFile,
	filePath string,
	allowNotFound bool,
) (string, error) {
	if _, statErr := os.Stat(filePath); os.IsNotExist(statErr) {
		if downloadErr := b.downloader.DownloadBundleFile(ctx, bundleFile, filePath, allowNotFound); downloadErr != nil {
			return "", fmt.Errorf("下载文件失败: %w", downloadErr)
		}
	}
	relPath, relErr := filepath.Rel(b.path, filePath)
	if relErr != nil {
		return "", fmt.Errorf("获取相对路径失败: %w", relErr)
	}
	return filepath.ToSlash(relPath), nil
}

// getFileType 根据文件路径判断文件类型
// 参数:
//   - filePath: 文件路径
//
// 返回:
//   - string: 文件类型（"model", "physics", "texture", "motion", "expression"）
func getFileType(filePath string) string {
	switch {
	case strings.HasSuffix(filePath, "model.moc"):
		return "model"
	case strings.HasSuffix(filePath, "physics.json"):
		return "physics"
	case strings.Contains(filePath, "textures"):
		return "texture"
	case strings.Contains(filePath, "motions"):
		return "motion"
	case strings.Contains(filePath, "expressions"):
		return "expression"
	default:
		return "unknown"
	}
}

// updateModelData 根据文件类型更新模型数据
// 参数:
//   - model: Live2D 模型
//   - filePath: 文件路径
//   - relPath: 相对路径
func updateModelData(model *model.Live2dModel, filePath, relPath string) {
	switch getFileType(filePath) {
	case "model":
		model.Model = relPath
	case "physics":
		model.Physics = relPath
	case "texture":
		model.Textures = append(model.Textures, relPath)
	case "motion":
		motionName := strings.Split(filepath.Base(filePath), ".")[0]
		model.Motions[motionName] = []MotionFile{{File: relPath}}
	case "expression":
		expressionName := strings.Split(filepath.Base(filePath), ".")[0]
		model.Expressions = append(model.Expressions, ExpressionFile{
			Name: expressionName,
			File: relPath,
		})
	}
}

// processExistingFiles 处理已存在的文件
// 参数:
//   - b: Live2D 构建器
//   - existingFiles: 已存在的文件列表
//
// 返回:
//   - int: 已处理的文件数量
//   - error: 错误信息
func (b *Live2dBuilder) processExistingFiles(existingFiles []string) (int, error) {
	completedFiles := 0
	for _, file := range existingFiles {
		relPath, err := filepath.Rel(b.path, file)
		if err != nil {
			return completedFiles, fmt.Errorf("获取相对路径失败: %w", err)
		}
		relPath = filepath.ToSlash(relPath)

		// 更新当前文件的进度
		completedFiles++
		if b.downloader.TuiModel != nil {
			b.downloader.TuiModel.UpdateProgress(b.ModelName, completedFiles)
		}

		// 更新模型数据
		updateModelData(b.model, file, relPath)
	}
	return completedFiles, nil
}

// createModelData 创建最终的模型数据
// 参数:
//   - b: Live2D 构建器
//
// 返回:
//   - error: 错误信息
func (b *Live2dBuilder) createModelData() error {
	modelData := model.Data{
		Version: "Sample 1.0.0",
		Layout: map[string]float64{
			"center_x": 0,
			"center_y": 0,
			"width":    2,
		},
		HitAreasCustom: map[string][]float64{
			"head_x": {-0.25, 1},
			"head_y": {0.25, 0.2},
			"body_x": {-0.3, 0.2},
			"body_y": {0.3, -1.9},
		},
		Model:       b.model.Model,
		Physics:     b.model.Physics,
		Textures:    b.model.Textures,
		Motions:     b.model.Motions,
		Expressions: b.model.Expressions,
	}

	log.DefaultLogger.Info().Str("modelName", b.ModelName).Msg("开始创建模型数据")

	finalJSON, err := json.MarshalIndent(modelData, "", "  ")
	if err != nil {
		log.DefaultLogger.Error().Str("modelName", b.ModelName).Err(err).Msg("序列化模型数据失败")
		if b.downloader.TuiModel != nil {
			b.downloader.TuiModel.SetError(fmt.Sprintf("%s: 创建模型数据失败: %v", b.ModelName, err))
		}
		return fmt.Errorf("序列化模型数据失败: %w", err)
	}

	modelJSONPath := filepath.Join(b.path, "model.json")
	if writeErr := os.WriteFile(modelJSONPath, finalJSON, 0600); writeErr != nil {
		log.DefaultLogger.Error().Str("modelName", b.ModelName).Str("path", modelJSONPath).Err(writeErr).Msg("写入模型数据失败")
		return fmt.Errorf("写入模型数据失败: %w", writeErr)
	}

	log.DefaultLogger.Info().Str("modelName", b.ModelName).Str("path", modelJSONPath).Msg("模型数据创建完成")
	return nil
}

// prepareDownloadTasks 准备下载任务列表
// 返回:
//   - []downloadTask: 下载任务列表
//   - []string: 已存在的文件列表
func (b *Live2dBuilder) prepareDownloadTasks() ([]downloadTask, []string) {
	var tasks []downloadTask
	var existingFiles []string

	// 模型文件
	modelFile := filepath.Join(b.dataPath, "model.moc")
	if _, err := os.Stat(modelFile); os.IsNotExist(err) {
		tasks = append(tasks, downloadTask{
			bundleFile:    b.data.Model,
			filePath:      modelFile,
			allowNotFound: false, // 模型文件必须存在
			result:        make(chan downloadResult, 1),
		})
	} else {
		existingFiles = append(existingFiles, modelFile)
	}

	// 物理文件
	physicsFile := filepath.Join(b.dataPath, "physics.json")
	if _, err := os.Stat(physicsFile); os.IsNotExist(err) {
		tasks = append(tasks, downloadTask{
			bundleFile:    b.data.Physics,
			filePath:      physicsFile,
			allowNotFound: true, // physics.json文件允许不存在
			result:        make(chan downloadResult, 1),
		})
	} else {
		existingFiles = append(existingFiles, physicsFile)
	}

	// 纹理文件
	texturePath := filepath.Join(b.dataPath, "textures")
	for _, texture := range b.data.Textures {
		file := filepath.Join(texturePath, texture.FileName)
		if _, err := os.Stat(file); os.IsNotExist(err) {
			tasks = append(tasks, downloadTask{
				bundleFile:    texture,
				filePath:      file,
				allowNotFound: false, // 纹理文件必须存在
				result:        make(chan downloadResult, 1),
			})
		} else {
			existingFiles = append(existingFiles, file)
		}
	}

	// 动作文件
	motionPath := filepath.Join(b.dataPath, "motions")
	for _, motion := range b.data.Motions {
		file := filepath.Join(motionPath, motion.FileName)
		if _, err := os.Stat(file); os.IsNotExist(err) {
			tasks = append(tasks, downloadTask{
				bundleFile:    motion,
				filePath:      file,
				allowNotFound: false, // 动作文件必须存在
				result:        make(chan downloadResult, 1),
			})
		} else {
			existingFiles = append(existingFiles, file)
		}
	}

	// 表情文件
	expressionPath := filepath.Join(b.dataPath, "expressions")
	for _, expression := range b.data.Expressions {
		file := filepath.Join(expressionPath, expression.FileName)
		if _, err := os.Stat(file); os.IsNotExist(err) {
			tasks = append(tasks, downloadTask{
				bundleFile:    expression,
				filePath:      file,
				allowNotFound: false, // 表情文件必须存在
				result:        make(chan downloadResult, 1),
			})
		} else {
			existingFiles = append(existingFiles, file)
		}
	}

	return tasks, existingFiles
}

// startWorkerPool 启动工作池处理下载任务
// 参数:
//   - ctx: 上下文
//   - taskChan: 任务通道
//   - errorChan: 错误通道
func (b *Live2dBuilder) startWorkerPool(ctx context.Context, taskChan chan downloadTask, errorChan chan error) {
	cfg := config.Get()
	for range cfg.MaxConcurrentDownloads {
		go func() {
			for task := range taskChan {
				select {
				case <-ctx.Done():
					errorChan <- errors.New("下载已取消")
					return
				default:
					if downloadErr := b.downloader.DownloadBundleFile(ctx, task.bundleFile, task.filePath, task.allowNotFound); downloadErr != nil {
						task.result <- downloadResult{err: fmt.Errorf("下载文件失败: %w", downloadErr)}
						continue
					}
					relPath, relErr := filepath.Rel(b.path, task.filePath)
					if relErr != nil {
						task.result <- downloadResult{err: fmt.Errorf("获取相对路径失败: %w", relErr)}
						continue
					}
					task.result <- downloadResult{relPath: filepath.ToSlash(relPath)}
				}
			}
		}()
	}
}

// processDownloadResults 处理下载结果
// 参数:
//   - ctx: 上下文
//   - tasks: 下载任务列表
//   - completedFiles: 已完成的文件数
//
// 返回:
//   - error: 错误信息
func (b *Live2dBuilder) processDownloadResults(ctx context.Context, tasks []downloadTask, completedFiles int) error {
	for i := range tasks {
		select {
		case <-ctx.Done():
			return errors.New("下载已取消")
		case result := <-tasks[i].result:
			if result.err != nil {
				// 直接返回第一个错误，不包裹
				return result.err
			}

			// 更新当前文件的进度
			completedFiles++
			if b.downloader.TuiModel != nil {
				b.downloader.TuiModel.UpdateProgress(b.ModelName, completedFiles)
			}

			// 更新模型数据
			updateModelData(b.model, tasks[i].filePath, result.relPath)
		}
	}
	return nil
}

// setupDownloadEnvironment 设置下载环境
// 包括上下文设置、信号量获取、目录创建等初始化工作.
func (b *Live2dBuilder) setupDownloadEnvironment() (context.Context, error) {
	// 设置上下文
	ctx := context.Background()
	if b.downloader.TuiModel != nil && b.downloader.TuiModel.Ctx != nil {
		ctx = b.downloader.TuiModel.Ctx
	}

	// 获取信号量
	select {
	case <-ctx.Done():
		log.DefaultLogger.Info().Str("modelName", b.ModelName).Msg("构建已取消")
		return nil, errors.New("下载已取消")
	case b.downloader.modelSem <- struct{}{}:
	}

	// 确保目录存在
	if err := os.MkdirAll(b.dataPath, 0750); err != nil {
		log.DefaultLogger.Error().Str("modelName", b.ModelName).Str("path", b.dataPath).Err(err).Msg("创建目录失败")
		if b.downloader.TuiModel != nil {
			b.downloader.TuiModel.SetError(fmt.Sprintf("%s: 创建目录失败: %v", b.ModelName, err))
		}
		<-b.downloader.modelSem // 释放信号量
		return nil, fmt.Errorf("创建目录失败: %w", err)
	}

	return ctx, nil
}

// initializeDownloadProgress 初始化下载进度.
func (b *Live2dBuilder) initializeDownloadProgress() {
	totalFiles := 1 + // model.moc
		1 + // physics.json
		len(b.data.Textures) + // textures
		len(b.data.Motions) + // motions
		len(b.data.Expressions) // expressions

	log.DefaultLogger.Info().Str("modelName", b.ModelName).Int("totalFiles", totalFiles).Msg("需要下载的文件总数")

	if b.downloader.TuiModel != nil {
		b.downloader.TuiModel.AddDownloadItem(b.ModelName, totalFiles)
	}
}

// handleDownloadTasks 处理下载任务.
func (b *Live2dBuilder) handleDownloadTasks(ctx context.Context, tasks []downloadTask, completedFiles int) error {
	if len(tasks) == 0 {
		return nil
	}

	taskChan := make(chan downloadTask, len(tasks))
	errorChan := make(chan error, 1)

	// 启动工作池
	b.startWorkerPool(ctx, taskChan, errorChan)

	// 发送所有任务
	for _, task := range tasks {
		select {
		case <-ctx.Done():
			return errors.New("下载已取消")
		case taskChan <- task:
		}
	}
	close(taskChan)

	// 处理下载结果
	if err := b.processDownloadResults(ctx, tasks, completedFiles); err != nil {
		if b.downloader.TuiModel != nil {
			b.downloader.TuiModel.SendError(b.ModelName, err)
		}
		return err
	}

	return nil
}

// Construct 构建完整的 Live2D 模型.
func (b *Live2dBuilder) Construct() error {
	log.DefaultLogger.Info().Str("modelName", b.ModelName).Msg("开始构建Live2D模型")

	// 设置下载环境
	ctx, err := b.setupDownloadEnvironment()
	if err != nil {
		return err
	}
	defer func() { <-b.downloader.modelSem }() // 完成后释放信号量

	// 初始化下载进度
	b.initializeDownloadProgress()

	// 准备下载任务
	tasks, existingFiles := b.prepareDownloadTasks()

	// 处理已存在的文件
	completedFiles, err := b.processExistingFiles(existingFiles)
	if err != nil {
		if b.downloader.TuiModel != nil {
			b.downloader.TuiModel.SendError(b.ModelName, err)
		}
		return err
	}

	// 处理下载任务
	if err = b.handleDownloadTasks(ctx, tasks, completedFiles); err != nil {
		return err
	}

	// 创建最终的模型数据
	return b.createModelData()
}

// GetAPIClient 获取API客户端实例
// 返回:
//   - *api.Client: API客户端实例
func (d *Downloader) GetAPIClient() *api.Client {
	return d.apiClient
}
