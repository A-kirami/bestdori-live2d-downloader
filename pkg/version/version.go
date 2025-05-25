// Package version 提供了版本信息
package version

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/suzuki-shunsuke/go-ci-env/cienv"
)

const (
	// Version 是应用程序的版本号.
	Version = "v1.2.0"
)

// GetVersion 返回版本信息.
func GetVersion() string {
	// 使用 go-ci-env 获取 CI 平台信息
	platform := cienv.Get()

	// 如果 platform 为 nil 或不是在 CI 环境中运行
	if platform == nil || !platform.Match() {
		return fmt.Sprintf("%s-dev", Version)
	}

	// 获取 Git 提交信息
	commit, err := getGitCommit()
	if err != nil {
		return fmt.Sprintf("%s-unknown", Version)
	}

	// 根据不同的 CI 平台处理版本号
	switch platform.CI() {
	case "github-actions":
		workflow := os.Getenv("GITHUB_WORKFLOW")
		switch workflow {
		case "Release":
			return Version
		case "Build":
			if platform.IsPR() {
				prNum, _ := platform.PRNumber()
				return fmt.Sprintf("%s-build.%s(pr#%d)", Version, commit[:7], prNum)
			}
			return fmt.Sprintf("%s-build.%s", Version, commit[:7])
		default:
			return fmt.Sprintf("%s-build.%s", Version, commit[:7])
		}
	default:
		return fmt.Sprintf("%s-build.%s", Version, commit[:7])
	}
}

// getGitCommit 获取当前 Git 提交的哈希值.
func getGitCommit() (string, error) {
	// 首先尝试从环境变量获取
	if sha := os.Getenv("GITHUB_BUILD_SHA"); sha != "" {
		return sha, nil
	}

	// 如果环境变量中没有，则从 Git 命令获取
	cmd := exec.Command("git", "rev-parse", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("执行 git rev-parse HEAD 失败: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}
