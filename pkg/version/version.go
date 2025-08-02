// Package version 提供了版本信息
package version

import "fmt"

//nolint:gochecknoglobals // 这些变量用于版本信息，是 GoReleaser 的标准做法
var (
	Version = "dev"
	Commit  = "none"
	BuiltBy = "unknown"
)

// GetVersionInfo 返回版本信息.
func GetVersionInfo() string {
	if BuiltBy != "goreleaser" {
		return Version
	}
	return fmt.Sprintf("%s-%s", Version, Commit)
}
