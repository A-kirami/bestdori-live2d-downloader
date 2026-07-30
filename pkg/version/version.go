// Package version 提供了版本信息
package version

const shortCommitLength = 7

//nolint:gochecknoglobals // 版本信息由 CI 和 GoReleaser 在构建时注入
var (
	Version = "dev"
	Commit  = "none"
)

// GetVersionInfo 返回版本信息.
func GetVersionInfo() string {
	if Commit == "" || Commit == "none" {
		return Version
	}

	commit := Commit
	if len(commit) > shortCommitLength {
		commit = commit[:shortCommitLength]
	}

	return Version + "-" + commit
}
