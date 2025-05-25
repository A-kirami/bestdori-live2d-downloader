package version_test

import (
	"os"
	"testing"

	"github.com/A-kirami/bestdori-live2d-downloader/pkg/version"
)

func TestGetVersion(t *testing.T) {
	// 保存原始环境变量
	originalCI := os.Getenv("CI")
	originalWorkflow := os.Getenv("GITHUB_WORKFLOW")
	originalBuildSHA := os.Getenv("GITHUB_BUILD_SHA")
	originalGithubActions := os.Getenv("GITHUB_ACTIONS")
	originalEventName := os.Getenv("GITHUB_EVENT_NAME")
	originalEventPath := os.Getenv("GITHUB_EVENT_PATH")

	// 恢复原始环境变量
	defer func() {
		t.Setenv("CI", originalCI)
		t.Setenv("GITHUB_WORKFLOW", originalWorkflow)
		t.Setenv("GITHUB_BUILD_SHA", originalBuildSHA)
		t.Setenv("GITHUB_ACTIONS", originalGithubActions)
		t.Setenv("GITHUB_EVENT_NAME", originalEventName)
		t.Setenv("GITHUB_EVENT_PATH", originalEventPath)
	}()

	tests := []struct {
		name            string
		ci              string
		workflow        string
		buildSHA        string
		githubActions   string
		eventName       string
		eventPath       string
		expectedVersion string
	}{
		{
			name:            "非 CI 环境",
			ci:              "",
			githubActions:   "",
			expectedVersion: version.Version + "-dev",
		},
		{
			name:            "Release 工作流",
			ci:              "true",
			workflow:        "Release",
			githubActions:   "true",
			expectedVersion: version.Version,
		},
		{
			name:            "Build 工作流带 PR",
			ci:              "true",
			workflow:        "Build",
			buildSHA:        "abcdef1234567890",
			githubActions:   "true",
			eventName:       "pull_request",
			eventPath:       "testdata/pull_request.json",
			expectedVersion: version.Version + "-build.abcdef1(pr#123)",
		},
		{
			name:            "Build 工作流无 PR",
			ci:              "true",
			workflow:        "Build",
			buildSHA:        "abcdef1234567890",
			githubActions:   "true",
			expectedVersion: version.Version + "-build.abcdef1",
		},
		{
			name:            "其他工作流",
			ci:              "true",
			workflow:        "Test",
			buildSHA:        "abcdef1234567890",
			githubActions:   "true",
			expectedVersion: version.Version + "-build.abcdef1",
		},
	}

	// 创建测试用的 PR 事件文件
	if err := os.MkdirAll("testdata", 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("testdata/pull_request.json", []byte(`{"pull_request":{"number":123}}`), 0644); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll("testdata")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("CI", tt.ci)
			t.Setenv("GITHUB_WORKFLOW", tt.workflow)
			t.Setenv("GITHUB_BUILD_SHA", tt.buildSHA)
			t.Setenv("GITHUB_ACTIONS", tt.githubActions)
			t.Setenv("GITHUB_EVENT_NAME", tt.eventName)
			t.Setenv("GITHUB_EVENT_PATH", tt.eventPath)

			got := version.GetVersion()
			if got != tt.expectedVersion {
				t.Errorf("GetVersion() = %v, want %v", got, tt.expectedVersion)
			}
		})
	}
}
