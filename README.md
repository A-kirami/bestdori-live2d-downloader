# Bestdori Live2D 下载器

一个用于从 Bestdori 下载 BanG Dream! 游戏中 Live2D 模型的命令行工具。本工具支持通过角色名称搜索和直接通过 Live2D 模型名称下载，并提供了友好的终端用户界面（TUI）。

## ✨ 主要特性

- 🎯 支持通过角色名称搜索 Live2D 模型
- 📥 支持直接通过 Live2D 模型名称下载
- 📁 自动处理模型文件结构
- ⚡ 支持批量下载多个 Live2D 模型
- 🎨 提供友好的终端用户界面

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
| `AssetsIndexURL` | 资源索引 API URL | `https://bestdori.com/api/assets` |
| `Live2dSavePath` | Live2D 模型保存路径 | `./live2d_download` |
| `LogPath` | 日志文件保存路径 | `./logs` |
| `UseCharaCache` | 是否使用角色信息缓存 | `true` |
| `CharaCachePath` | 角色信息缓存路径 | `./live2d_chara_cache` |
| `CacheDuration` | 缓存过期时间 | `24h` |
| `MaxConcurrentDownloads` | 单个模型下载时的最大并发文件下载数 | `5` |
| `MaxConcurrentModels` | 最大并发模型下载数 | `3` |

## 📖 使用方法

1. 运行程序：

   ```bash
   # Windows
   .\bestdori-live2d-downloader.exe

   # Linux/macOS
   ./bestdori-live2d-downloader
   ```

2. 输入角色名称或 Live2D 名称：
   - 输入角色名称（如 "爱音"）将搜索并列出该角色的所有 Live2D 模型
   - 输入 Live2D 模型名称（如 "037_casual-2023"）将直接下载指定的模型

3. 下载的模型将保存在配置的 `Live2dSavePath` 目录中，按照以下结构组织：

   ```text
   Live2dSavePath/
   └── 角色名/
         └── 模型名/
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
2. 创建您的特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交您的更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 开启一个 Pull Request

## 🙏 致谢

- [Bestdori](https://bestdori.com/) - 提供 Live2D 模型资源
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - 提供终端用户界面框架

## 📄 许可证

Code: MIT, 2025, Akirami
