// Package model 定义了 Live2D 模型相关的数据结构
// 包括资源包文件、构建数据、动作文件、表情文件等类型
package model

import "strings"

// BundleFile 表示资源包文件
// 用于描述从 Bestdori 下载的资源文件信息.
type BundleFile struct {
	BundleName string `json:"bundleName"` // 资源包名称
	FileName   string `json:"fileName"`   // 文件名
}

// ProcessFileName 处理文件名，移除 .bytes 后缀
// 用于清理从 Bestdori 下载的文件名.
func (b *BundleFile) ProcessFileName() {
	b.FileName = strings.TrimSuffix(b.FileName, ".bytes")
}

// BuildData 表示 Live2D 模型的构建数据
// 包含模型所需的所有文件信息.
type BuildData struct {
	Model       BundleFile   `json:"model"`       // 模型文件
	Physics     BundleFile   `json:"physics"`     // 物理文件
	Textures    []BundleFile `json:"textures"`    // 纹理文件列表
	Transition  BundleFile   `json:"transition"`  // 过渡文件
	Motions     []BundleFile `json:"motions"`     // 动作文件列表
	Expressions []BundleFile `json:"expressions"` // 表情文件列表
}

// MotionFile 表示动作文件
// 用于描述 Live2D 模型的动作信息.
type MotionFile struct {
	File string `json:"file"` // 动作文件路径
}

// ExpressionFile 表示表情文件
// 用于描述 Live2D 模型的表情信息.
type ExpressionFile struct {
	Name string `json:"name"` // 表情名称
	File string `json:"file"` // 表情文件路径
}

// Live2dModel 表示完整的 Live2D 模型
// 包含模型的所有组件信息.
type Live2dModel struct {
	Model       string                  `json:"model,omitempty"`       // 模型文件路径
	Physics     string                  `json:"physics,omitempty"`     // 物理文件路径
	Textures    []string                `json:"textures,omitempty"`    // 纹理文件路径列表
	Motions     map[string][]MotionFile `json:"motions,omitempty"`     // 动作文件映射
	Expressions []ExpressionFile        `json:"expressions,omitempty"` // 表情文件列表
}

// Data 表示 Live2D 模型的数据结构.
type Data struct {
	Version        string                  `json:"version"`
	Layout         map[string]float64      `json:"layout"`
	HitAreasCustom map[string][]float64    `json:"hit_areas_custom"`
	Model          string                  `json:"model"`
	Physics        string                  `json:"physics"`
	Textures       []string                `json:"textures"`
	Motions        map[string][]MotionFile `json:"motions"`
	Expressions    []ExpressionFile        `json:"expressions"`
}

// MatchChara 表示匹配的角色信息
// 用于存储角色搜索的结果.
type MatchChara struct {
	ID    int      `json:"id"`    // 角色ID
	Name  string   `json:"name"`  // 角色名称
	Names []string `json:"names"` // 角色所有可能的名称列表
}
