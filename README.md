# Bestdori Live2D 下载器

一个用于从 Bestdori 下载 BanG Dream! 游戏中 Live2D 模型的命令行工具。支持角色列表选择、服装名称翻译、搜索过滤，并提供了友好的终端用户界面（TUI）。

## ✨ 主要特性

- 🎨 **角色列表选择** - 按乐队 Tab 分类，并支持角色颜色显示
- 🌐 **服装名称自动翻译** - 优先显示简体中文，其次繁体中文、日语、英语
- 🔍 **服装搜索过滤** - 支持中文输入法，实时过滤搜索结果
- 📝 **命名模式切换** - 支持中文命名和原始文件名切换
- 📁 **智能文件夹命名** - 自动使用角色名和服装名创建文件夹
- 🔄 **重复文件检测** - 自动检测已下载文件，避免重复下载
- ⚡ **批量下载** - 支持选择多个服装批量下载
- 🎯 **美观易用的 TUI** - 统一的界面风格，清晰的操作提示

## 🚀 快速开始

### 直接使用

1. 从 [Releases](https://github.com/A-kirami/bestdori-live2d-downloader/releases) 页面下载最新版本的可执行文件
2. 运行程序：

   ```bash
   # Windows
   .\bestdori-live2d-downloader.exe

   # Linux/macOS
   ./bestdori-live2d-downloader
   ```

### 从源码构建

1. 确保已安装 Go 1.23.4 或更高版本
2. 克隆仓库：

   ```bash
   git clone https://github.com/A-kirami/bestdori-live2d-downloader.git
   cd bestdori-live2d-downloader
   ```

3. 安装依赖：

   ```bash
   go mod download
   ```

4. 编译程序：

   ```bash
   # Windows
   go build -o bestdori-live2d-downloader.exe cmd/bestdori-live2d-downloader/main.go

   # Linux/macOS
   go build -o bestdori-live2d-downloader cmd/bestdori-live2d-downloader/main.go
   ```

## ⚙️ 配置说明

程序使用统一的配置系统，所有配置项都集中在 `pkg/config/config.go` 中管理。主要配置项包括：

| 配置项 | 说明 | 默认值 |
|--------|------|--------|
| `BaseAssetsURL` | Bestdori 资源基础 URL | `https://bestdori.com/assets/` |
| `CharaRosterURL` | 角色信息 API URL | `https://bestdori.com/api/characters` |
| `BandListURL` | 乐队信息 API URL | `https://bestdori.com/api/bands/main.1.json` |
| `AssetsIndexURL` | 资源索引 API URL | `https://bestdori.com/api/assets` |
| `Live2dSavePath` | Live2D 模型保存路径 | `./live2d_download` |
| `LogPath` | 日志文件保存路径 | `./logs` |
| `UseCharaCache` | 是否使用角色信息缓存 | `true` |
| `CharaCachePath` | 角色信息缓存路径 | `./live2d_chara_cache` |
| `CacheDuration` | 缓存过期时间 | `24h` |
| `MaxConcurrentDownloads` | 单个模型下载时的最大并发文件下载数 | `20` |
| `MaxConcurrentModels` | 最大并发模型下载数 | `3` |
| `NamingMode` | 文件夹命名模式（0=中文，1=原始） | `0` |

## 📖 使用方法

1. 运行程序：

   ```bash
   # Windows
   .\bestdori-live2d-downloader.exe

   # Linux/macOS
   ./bestdori-live2d-downloader
   ```

2. **选择角色**：使用左右键或 `Tab` 切换乐队，使用上下键选择角色，按 `Enter` 确认

3. **选择服装**：
   - 使用空格键选择/取消选择服装
   - 按 `A` 全选/取消全选
   - 按 `N` 切换命名模式（中文/原始）
   - 按 `/` 进入搜索模式，输入关键词过滤
   - 按 `Enter` 开始下载

4. **下载完成**：按 `Esc` 返回服装列表

5. 下载的模型将保存在配置的 `Live2dSavePath` 目录中，按照以下结构组织：

   ```text
   Live2dSavePath/
   └── 角色名/
         └── 服装名/
            ├── data/
            │   ├── model.moc
            │   ├── physics.json
            │   ├── textures/
            │   ├── motions/
            │   └── expressions/
            └── model.json
   ```

## 🤝 贡献指南

1. Fork 本仓库
2. 创建你的特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交你的更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 开启一个 Pull Request

## 🙏 致谢

- [Bestdori](https://bestdori.com/) - 提供 Live2D 模型资源
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - 提供终端用户界面框架

## 📄 许可证

Code: MIT, 2025, Akirami
