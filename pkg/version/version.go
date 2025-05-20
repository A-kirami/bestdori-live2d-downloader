// Package version 提供了版本信息
package version

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const (
	// Version 是应用程序的版本号.
	Version = "v0.0.0"
)

// GetVersion 返回版本信息.
func GetVersion() string {
	// 非 CI 环境直接返回开发版本
	if os.Getenv("CI") != "true" {
		return fmt.Sprintf("%s-dev", Version)
	}

	// 获取 Git 提交信息
	commit, err := getGitCommit()
	if err != nil {
		return fmt.Sprintf("%s-unknown", Version)
	}

	// 获取工作流信息
	workflow := os.Getenv("GITHUB_WORKFLOW")
	switch workflow {
	case "Release":
		return Version
	case "Build":
		prNum := os.Getenv("GITHUB_PR_NUMBER")
		if prNum != "" {
			return fmt.Sprintf("%s-build.%s (pr#%s)", Version, commit[:7], prNum)
		}
		return fmt.Sprintf("%s-build.%s", Version, commit[:7])
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
