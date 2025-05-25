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
	originalPRNum := os.Getenv("GITHUB_PR_NUMBER")
	originalBuildSHA := os.Getenv("GITHUB_BUILD_SHA")

	// 恢复原始环境变量
	defer func() {
		t.Setenv("CI", originalCI)
		t.Setenv("GITHUB_WORKFLOW", originalWorkflow)
		t.Setenv("GITHUB_PR_NUMBER", originalPRNum)
		t.Setenv("GITHUB_BUILD_SHA", originalBuildSHA)
	}()

	tests := []struct {
		name           string
		ci             string
		workflow       string
		prNum          string
		buildSHA       string
		expectedPrefix string
	}{
		{
			name:           "非 CI 环境",
			ci:             "",
			expectedPrefix: version.Version + "-dev",
		},
		{
			name:           "Release 工作流",
			ci:             "true",
			workflow:       "Release",
			expectedPrefix: version.Version,
		},
		{
			name:           "Build 工作流带 PR",
			ci:             "true",
			workflow:       "Build",
			prNum:          "123",
			buildSHA:       "abcdef1234567890",
			expectedPrefix: version.Version + "-build.abcdef1 (pr#123)",
		},
		{
			name:           "Build 工作流无 PR",
			ci:             "true",
			workflow:       "Build",
			buildSHA:       "abcdef1234567890",
			expectedPrefix: version.Version + "-build.abcdef1",
		},
		{
			name:           "其他工作流",
			ci:             "true",
			workflow:       "Test",
			buildSHA:       "abcdef1234567890",
			expectedPrefix: version.Version + "-build.abcdef1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("CI", tt.ci)
			t.Setenv("GITHUB_WORKFLOW", tt.workflow)
			t.Setenv("GITHUB_PR_NUMBER", tt.prNum)
			t.Setenv("GITHUB_BUILD_SHA", tt.buildSHA)

			got := version.GetVersion()
			if got != tt.expectedPrefix {
				t.Errorf("GetVersion() = %v, want %v", got, tt.expectedPrefix)
			}
		})
	}
}
