package version_test

import (
	"testing"

	"github.com/A-kirami/bestdori-live2d-downloader/pkg/version"
	"github.com/stretchr/testify/require"
)

func TestGetVersionInfoReturnsReleaseVersionWithoutCommit(t *testing.T) {
	originalVersion := version.Version
	originalCommit := version.Commit
	t.Cleanup(func() {
		//nolint:reassign // 恢复测试前的构建版本值
		version.Version = originalVersion
		//nolint:reassign // 恢复测试前的构建提交值
		version.Commit = originalCommit
	})
	//nolint:reassign // 模拟 GoReleaser 通过 linker 注入版本
	version.Version = "1.6.0"
	//nolint:reassign // 模拟 GoReleaser 未注入提交值
	version.Commit = "none"

	require.Equal(t, "1.6.0", version.GetVersionInfo())
}

func TestGetVersionInfoIncludesShortCommitForBuild(t *testing.T) {
	originalVersion := version.Version
	originalCommit := version.Commit
	t.Cleanup(func() {
		//nolint:reassign // 恢复测试前的构建版本值
		version.Version = originalVersion
		//nolint:reassign // 恢复测试前的构建提交值
		version.Commit = originalCommit
	})
	//nolint:reassign // 模拟 CI 通过 linker 注入提交值
	version.Version = "dev"
	//nolint:reassign // 模拟 CI 通过 linker 注入提交值
	version.Commit = "abcdef0123456789"

	require.Equal(t, "dev-abcdef0", version.GetVersionInfo())
}
