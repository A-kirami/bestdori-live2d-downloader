// Package tui 提供了终端用户界面（TUI）的实现
// 包括文本输入、列表显示、进度条、下载状态等功能
package tui

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/A-kirami/bestdori-live2d-downloader/pkg/version"

	"slices"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// 全局样式定义.
var (
	//nolint:gochecknoglobals // 使用全局样式常量是必要的，因为需要在不同的 UI 组件中保持一致的样式
	helpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#626262")).Render // 帮助文本样式
	//nolint:gochecknoglobals // 使用全局样式常量是必要的，因为需要在不同的 UI 组件中保持一致的样式
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FF69B4")) // 标题样式
)

// 界面常量.
const (
	padding  = 2  // 内边距
	maxWidth = 80 // 最大宽度

	// 状态常量.
	StateInput       = "input"       // 输入状态
	StateList        = "list"        // 列表状态
	StateLoading     = "loading"     // 加载状态
	StateDownloading = "downloading" // 下载状态
	KeyEsc           = "esc"         // ESC 键
)

// progressMsg 表示进度更新消息.
type progressMsg struct {
	itemName string  // 项目名称
	ratio    float64 // 进度比例
}

// progressErrMsg 表示进度错误消息.
type progressErrMsg struct {
	itemName string // 项目名称
	err      error  // 错误信息
}

// DownloadItem 表示下载项.
type DownloadItem struct {
	Name     string         // 项目名称
	Progress progress.Model // 进度条模型
	Total    int            // 总文件数
	Current  int            // 当前完成数
	Err      error          // 错误信息
}

// DownloadListItem 表示下载列表项.
type DownloadListItem struct {
	Name     string         // 项目名称
	Progress progress.Model // 进度条模型
	Total    int            // 总文件数
	Current  int            // 当前完成数
	Err      error          // 错误信息
}

// Title 返回下载列表项的标题.
func (i DownloadListItem) Title() string {
	progress := float64(i.Current) / float64(i.Total)
	progressStr := fmt.Sprintf("%.1f%%", progress*100)
	if i.Err != nil {
		return fmt.Sprintf("❌ %s (%s) - 错误: %v", i.Name, progressStr, i.Err)
	}
	if i.Current == i.Total {
		return fmt.Sprintf("✅ %s (%s)", i.Name, progressStr)
	}
	return fmt.Sprintf("⏳ %s (%s)", i.Name, progressStr)
}

// Description 返回下载列表项的描述.
func (i DownloadListItem) Description() string {
	return i.Progress.ViewAs(i.Progress.Percent())
}

// FilterValue 返回用于过滤的值.
func (i DownloadListItem) FilterValue() string { return i.Name }

// listItem 表示列表项.
type listItem struct {
	title    string // 标题
	selected bool   // 是否选中
}

// Title 返回列表项的标题.
func (i listItem) Title() string {
	if i.selected {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#FF69B4")).Render("✓ " + i.title)
	}
	return "  " + i.title
}

// Description 返回列表项的描述.
func (i listItem) Description() string { return "" }

// FilterValue 返回用于过滤的值.
func (i listItem) FilterValue() string { return i.title }

// Model 表示 TUI 模型
// 包含所有 UI 组件和状态.
type Model struct {
	Items            map[string]*DownloadItem // 下载项映射，key 为项目名称，value 为下载项
	ItemOrder        []string                 // 下载项顺序列表
	Width            int                      // 界面宽度
	Quitting         bool                     // 是否正在退出程序
	TextInput        textinput.Model          // 文本输入框组件
	Live2dList       list.Model               // Live2D 列表组件
	DownloadList     list.Model               // 下载列表组件
	SelectedIDs      []int                    // 选中的项目 ID 列表
	State            string                   // 当前状态
	SearchChan       chan string              // 搜索通道，用于处理搜索请求
	SelectChan       chan []string            // 选择通道，用于处理选择请求
	Spinner          spinner.Model            // 加载动画组件
	CurrentCharaName string                   // 当前角色名称
	ExtraCharaName   string                   // 额外角色名称
	program          *tea.Program             // TUI 程序实例
	cancelChan       chan struct{}            // 取消通道，用于取消操作
	Ctx              context.Context          // 上下文，用于控制操作的生命周期
	Cancel           context.CancelFunc       // 取消函数，用于取消上下文
	ErrorMessage     string                   // 错误消息
}

// DownloadDelegate 用于下载进度列表的代理
// 自定义列表项的渲染方式.
type DownloadDelegate struct{}

// Height 返回列表项的高度.
func (d DownloadDelegate) Height() int { return 2 }

// Spacing 返回列表项的间距.
func (d DownloadDelegate) Spacing() int { return 1 }

// Update 处理列表项的更新.
func (d DownloadDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

// Render 渲染列表项.
func (d DownloadDelegate) Render(w io.Writer, _ list.Model, _ int, item list.Item) {
	dl, ok := item.(DownloadListItem)
	if !ok {
		return
	}
	title := dl.Title()
	desc := dl.Description()
	fmt.Fprintf(w, "  %s\n  %s", title, desc)
}

// NewModel 创建新的 TUI 模型实例.
func NewModel() Model {
	ctx, cancel := context.WithCancel(context.Background())

	ti := textinput.New()
	ti.Placeholder = "输入角色名称或 Live2D 模型名称"
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 50

	// 创建自定义的列表样式
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false

	l := list.New([]list.Item{}, delegate, 0, 0)
	l.Title = "选择要下载的 Live2D 模型"
	l.SetShowHelp(true)
	l.DisableQuitKeybindings()
	l.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(
				key.WithKeys("space"),
				key.WithHelp("space", "选择/取消选择"),
			),
			key.NewBinding(
				key.WithKeys("a"),
				key.WithHelp("a", "全选/取消全选"),
			),
		}
	}

	// 创建下载列表，使用自定义 DownloadDelegate
	downloadDelegate := DownloadDelegate{}
	downloadList := list.New([]list.Item{}, downloadDelegate, 0, 0)
	downloadList.Title = "下载进度"
	downloadList.SetShowHelp(true)
	downloadList.DisableQuitKeybindings()

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF69B4"))

	return Model{
		Items:        make(map[string]*DownloadItem),
		ItemOrder:    []string{},
		TextInput:    ti,
		Live2dList:   l,
		DownloadList: downloadList,
		State:        StateInput,
		SearchChan:   make(chan string, 1),
		SelectChan:   make(chan []string, 1),
		Spinner:      s,
		cancelChan:   make(chan struct{}), // 初始化取消通道
		Ctx:          ctx,
		Cancel:       cancel,
	}
}

// Init 初始化 TUI 模型.
func (m *Model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.Spinner.Tick)
}

// UpdateListMsg 表示更新列表消息.
type UpdateListMsg struct {
	Items []string // 列表项
}

// UpdateDownloadListMsg 表示更新下载列表消息.
type UpdateDownloadListMsg struct {
	Items []DownloadListItem // 下载列表项
}

// handleInputState 处理输入状态下的消息.
func (m *Model) handleInputState(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "enter" {
		value := strings.TrimSpace(m.TextInput.Value())
		if value == "" {
			m.SetError("请输入角色名称或 Live2D 模型名称")
			return m, nil
		}
		m.State = StateLoading
		select {
		case m.SearchChan <- value:
		default:
		}
		return m, m.Spinner.Tick
	}
	var cmd tea.Cmd
	m.TextInput, cmd = m.TextInput.Update(msg)
	return m, cmd
}

// handleLoadingState 处理加载状态下的消息.
func (m *Model) handleLoadingState(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == KeyEsc {
		m.State = StateInput
		return m, nil
	}
	return m, nil
}

// handleListState 处理列表状态下的消息.
func (m *Model) handleListState(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case " ":
		if i, ok := m.Live2dList.SelectedItem().(listItem); ok {
			i.selected = !i.selected
			if i.selected {
				m.SelectedIDs = append(m.SelectedIDs, m.Live2dList.Index())
			} else {
				for j, id := range m.SelectedIDs {
					if id == m.Live2dList.Index() {
						m.SelectedIDs = slices.Delete(m.SelectedIDs, j, j+1)
						break
					}
				}
			}
			m.Live2dList.SetItem(m.Live2dList.Index(), i)
		}
	case "a":
		m.handleSelectAll()
	case "up":
		if m.Live2dList.Index() == 0 && len(m.Live2dList.Items()) > 0 {
			m.Live2dList.Select(len(m.Live2dList.Items()) - 1)
			return m, nil
		}
	case "down":
		if m.Live2dList.Index() == len(m.Live2dList.Items())-1 && len(m.Live2dList.Items()) > 0 {
			m.Live2dList.Select(0)
			return m, nil
		}
	case "enter":
		return m.handleListEnter()
	case KeyEsc:
		m.State = StateInput
		m.Live2dList.Select(0)
		// 清空下载项
		m.Items = make(map[string]*DownloadItem)
		m.ItemOrder = []string{}
		m.updateDownloadList()
		// 重置输入框
		m.TextInput.Reset()
		return m, nil
	}
	var cmd tea.Cmd
	m.Live2dList, cmd = m.Live2dList.Update(msg)
	return m, cmd
}

// handleSelectAll 处理全选/取消全选.
func (m *Model) handleSelectAll() {
	allSelected := true
	for _, i := range m.Live2dList.Items() {
		item, ok := i.(listItem)
		if !ok {
			continue
		}
		if !item.selected {
			allSelected = false
			break
		}
	}
	for i, item := range m.Live2dList.Items() {
		it, ok := item.(listItem)
		if !ok {
			continue
		}
		it.selected = !allSelected
		m.Live2dList.SetItem(i, it)
	}
	if !allSelected {
		m.SelectedIDs = make([]int, len(m.Live2dList.Items()))
		for i := range m.Live2dList.Items() {
			m.SelectedIDs[i] = i
		}
	} else {
		m.SelectedIDs = nil
	}
}

// handleListEnter 处理列表状态下的回车键.
func (m *Model) handleListEnter() (tea.Model, tea.Cmd) {
	selected := m.GetSelectedItems()
	if len(selected) > 0 {
		for _, name := range selected {
			m.AddDownloadItem(name, 1)
		}
		m.State = StateDownloading
		if m.CurrentCharaName != "" {
			title := fmt.Sprintf("下载进度 - %s", m.CurrentCharaName)
			if m.ExtraCharaName != "" {
				title = fmt.Sprintf("%s (%s)", title, m.ExtraCharaName)
			}
			m.DownloadList.Title = title
		} else {
			m.DownloadList.Title = "下载进度"
		}
		select {
		case m.SelectChan <- selected:
		default:
		}
	}
	return m, nil
}

// handleDownloadingState 处理下载状态下的消息.
func (m *Model) handleDownloadingState(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up":
		if m.DownloadList.Index() == 0 && len(m.DownloadList.Items()) > 0 {
			m.DownloadList.Select(len(m.DownloadList.Items()) - 1)
			return m, nil
		}
	case "down":
		if m.DownloadList.Index() == len(m.DownloadList.Items())-1 && len(m.DownloadList.Items()) > 0 {
			m.DownloadList.Select(0)
			return m, nil
		}
	case KeyEsc:
		m.State = StateInput
		// 清空下载项
		m.Items = make(map[string]*DownloadItem)
		m.ItemOrder = []string{}
		m.updateDownloadList()
		// 重置输入框和列表光标
		m.TextInput.Reset()
		m.Live2dList.Select(0)
		return m, nil
	}
	var cmd tea.Cmd
	m.DownloadList, cmd = m.DownloadList.Update(msg)
	return m, cmd
}

// handleUpdateListMsg 处理更新列表消息.
func (m *Model) handleUpdateListMsg(msg UpdateListMsg) (tea.Model, tea.Cmd) {
	listItems := make([]list.Item, len(msg.Items))
	for i, item := range msg.Items {
		listItems[i] = listItem{
			title:    item,
			selected: false,
		}
	}
	m.Live2dList.SetItems(listItems)
	m.SelectedIDs = nil
	m.State = StateList
	if m.CurrentCharaName != "" {
		title := fmt.Sprintf("选择要下载的 Live2D 模型 - %s", m.CurrentCharaName)
		if m.ExtraCharaName != "" {
			title = fmt.Sprintf("%s (%s)", title, m.ExtraCharaName)
		}
		m.Live2dList.Title = title
	} else {
		m.Live2dList.Title = "选择要下载的 Live2D 模型"
	}
	return m, nil
}

// handleUpdateDownloadListMsg 处理更新下载列表消息.
func (m *Model) handleUpdateDownloadListMsg(msg UpdateDownloadListMsg) (tea.Model, tea.Cmd) {
	listItems := make([]list.Item, len(msg.Items))
	for i, item := range msg.Items {
		listItems[i] = item
	}
	m.DownloadList.SetItems(listItems)
	return m, nil
}

// handleKeyMsg 处理键盘消息.
func (m *Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" || (msg.String() == KeyEsc && m.State == StateInput) {
		close(m.cancelChan)
		m.Cancel()
		m.Quitting = true
		return m, tea.Quit
	}

	switch m.State {
	case StateInput:
		return m.handleInputState(msg)
	case StateLoading:
		return m.handleLoadingState(msg)
	case StateList:
		return m.handleListState(msg)
	case StateDownloading:
		return m.handleDownloadingState(msg)
	}

	return m, nil
}

// handleWindowSizeMsg 处理窗口大小消息.
func (m *Model) handleWindowSizeMsg(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.Width = msg.Width - padding*2 - 4
	if m.Width > maxWidth {
		m.Width = maxWidth
	}
	for _, item := range m.Items {
		item.Progress.Width = m.Width
	}
	availableHeight := msg.Height - padding*2 - 6
	m.Live2dList.SetWidth(msg.Width - padding*2)
	m.Live2dList.SetHeight(availableHeight)
	m.DownloadList.SetWidth(msg.Width - padding*2)
	m.DownloadList.SetHeight(availableHeight)
	return m, nil
}

// handleProgressMsg 处理进度消息.
func (m *Model) handleProgressMsg(msg progressMsg) (tea.Model, tea.Cmd) {
	item, exists := m.Items[msg.itemName]
	if !exists {
		item = &DownloadItem{
			Name:     msg.itemName,
			Progress: progress.New(progress.WithDefaultGradient()),
			Total:    1,
		}
		item.Progress.Width = m.Width
		m.Items[msg.itemName] = item
	}

	cmd := item.Progress.SetPercent(msg.ratio)
	m.updateDownloadList()
	return m, cmd
}

// handleProgressErrMsg 处理进度错误消息.
func (m *Model) handleProgressErrMsg(msg progressErrMsg) (tea.Model, tea.Cmd) {
	if item, exists := m.Items[msg.itemName]; exists {
		item.Err = msg.err
		m.updateDownloadList()
	}
	return m, nil
}

// handleProgressFrameMsg 处理进度帧消息.
func (m *Model) handleProgressFrameMsg(msg progress.FrameMsg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	for _, item := range m.Items {
		progressModel, cmd := item.Progress.Update(msg)
		if progressModel, ok := progressModel.(progress.Model); ok {
			item.Progress = progressModel
			cmds = append(cmds, cmd)
		}
	}
	return m, tea.Batch(cmds...)
}

// Update 处理 TUI 模型的更新.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case UpdateListMsg:
		return m.handleUpdateListMsg(msg)
	case UpdateDownloadListMsg:
		return m.handleUpdateDownloadListMsg(msg)
	case tea.KeyMsg:
		return m.handleKeyMsg(msg)
	case tea.WindowSizeMsg:
		return m.handleWindowSizeMsg(msg)
	case progressMsg:
		return m.handleProgressMsg(msg)
	case progressErrMsg:
		return m.handleProgressErrMsg(msg)
	case progress.FrameMsg:
		return m.handleProgressFrameMsg(msg)
	}

	if m.State == StateLoading {
		var cmd tea.Cmd
		m.Spinner, cmd = m.Spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) View() string {
	if m.Quitting {
		return "\n  下载已取消\n\n"
	}

	var s strings.Builder
	s.WriteString("\n")
	s.WriteString(titleStyle.Render("Bestdori Live2D 下载器"))
	s.WriteString("\n")
	s.WriteString(helpStyle(fmt.Sprintf("%s | 作者: Akirami", version.GetVersion())))
	s.WriteString("\n\n")

	switch m.State {
	case StateInput:
		s.WriteString(m.TextInput.View())
		s.WriteString("\n\n")
		if m.ErrorMessage != "" {
			s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")).Render(m.ErrorMessage))
			s.WriteString("\n\n")
		}
		s.WriteString(helpStyle("按 Enter 确认，按 Esc 或 Ctrl+C 退出"))

	case StateLoading:
		s.WriteString(m.TextInput.View())
		s.WriteString("\n\n")
		s.WriteString(fmt.Sprintf("%s 正在搜索角色...", m.Spinner.View()))
		s.WriteString("\n\n")
		s.WriteString(helpStyle("按 Esc 或 Ctrl+C 退出"))

	case StateList:
		s.WriteString(m.Live2dList.View())
		s.WriteString("\n\n")
		s.WriteString(helpStyle("使用空格选择/取消选择，A 全选/取消全选，Enter 确认，Esc 返回，Ctrl+C 退出"))

	case StateDownloading:
		s.WriteString(m.DownloadList.View())
		s.WriteString("\n\n")
		s.WriteString(helpStyle("按 Esc 返回主菜单，Ctrl+C 退出"))
	}

	return s.String()
}

func (m *Model) AddDownloadItem(name string, totalFiles int) {
	// 检查是否已存在相同名称的下载项
	if item, exists := m.Items[name]; exists {
		// 如果已存在，更新总数和重置进度
		item.Total = totalFiles
		item.Current = 0 // 重置当前进度
		m.updateDownloadList()
		return
	}

	item := &DownloadItem{
		Name:     name,
		Progress: progress.New(progress.WithDefaultGradient()),
		Total:    totalFiles,
		Current:  0,
	}
	if m.Width > 0 {
		item.Progress.Width = m.Width
	}
	m.Items[name] = item
	m.ItemOrder = append(m.ItemOrder, name)
	m.updateDownloadList()
}

func (m *Model) UpdateProgress(name string, current int) {
	select {
	case <-m.Ctx.Done():
		return
	case <-m.cancelChan:
		return
	default:
		if item, exists := m.Items[name]; exists {
			item.Current = current
			ratio := float64(item.Current) / float64(item.Total)
			m.program.Send(progressMsg{
				itemName: name,
				ratio:    ratio,
			})
		}
	}
}

func (m *Model) SetError(message string) {
	m.ErrorMessage = message
}

func (m *Model) ClearError() {
	m.ErrorMessage = ""
}

func (m *Model) updateDownloadList() {
	items := make([]list.Item, 0, len(m.Items))
	// 按照 ItemOrder 的顺序添加下载项
	for _, name := range m.ItemOrder {
		if item, exists := m.Items[name]; exists {
			items = append(items, DownloadListItem{
				Name:     item.Name,
				Progress: item.Progress,
				Total:    item.Total,
				Current:  item.Current,
				Err:      item.Err,
			})
		}
	}
	m.DownloadList.SetItems(items)
}

func (m *Model) SetLive2DList(items []string) {
	listItems := make([]list.Item, len(items))
	for i, item := range items {
		listItems[i] = listItem{
			title:    item,
			selected: false,
		}
	}
	m.Live2dList.SetItems(listItems)
	m.SelectedIDs = nil
	// 设置列表状态
	m.State = StateList
}

func (m *Model) GetSelectedItems() []string {
	// 使用 map 来确保唯一性
	uniqueItems := make(map[string]struct{})
	for _, id := range m.SelectedIDs {
		if id < len(m.Live2dList.Items()) {
			if item, ok := m.Live2dList.Items()[id].(listItem); ok {
				uniqueItems[item.title] = struct{}{}
			}
		}
	}

	// 将 map 转换回切片
	selected := make([]string, 0, len(uniqueItems))
	for item := range uniqueItems {
		selected = append(selected, item)
	}

	// 对选中的项目进行排序
	sort.Slice(selected, func(i, j int) bool {
		// 提取服装ID（模型名称中的数字部分）
		iParts := strings.Split(selected[i], "_")
		jParts := strings.Split(selected[j], "_")

		// 如果包含"live_event"，将其排在后面
		iHasEvent := strings.Contains(selected[i], "live_event")
		jHasEvent := strings.Contains(selected[j], "live_event")

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
		return selected[i] < selected[j]
	})

	return selected
}

func (m *Model) GetSearchChan() <-chan string {
	return m.SearchChan
}

func (m *Model) GetSelectChan() <-chan []string {
	return m.SelectChan
}

// GetCancelChan 返回取消通道.
func (m *Model) GetCancelChan() <-chan struct{} {
	return m.cancelChan
}

// SetProgram 设置程序实例.
func (m *Model) SetProgram(p *tea.Program) {
	m.program = p
}

// SendError 发送错误消息.
func (m *Model) SendError(itemName string, err error) {
	if m.program != nil {
		m.program.Send(progressErrMsg{
			itemName: itemName,
			err:      err,
		})
	}
}
