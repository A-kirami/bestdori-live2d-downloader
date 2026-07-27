package downloader

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/A-kirami/bestdori-live2d-downloader/pkg/model"
	"github.com/stretchr/testify/require"
)

func TestPrepareDownloadTasksRejectsDirectoryAsExistingFile(t *testing.T) {
	dataPath := filepath.Join(t.TempDir(), "data")
	modelPath := filepath.Join(dataPath, "model.moc")
	require.NoError(t, os.MkdirAll(modelPath, 0750))

	builder := &Live2dBuilder{
		dataPath: dataPath,
		data: &model.BuildData{
			Model: model.BundleFile{FileName: "model.moc"},
		},
	}

	tasks, existingFiles := builder.prepareDownloadTasks()

	require.NotContains(t, existingFiles, modelPath)
	require.Condition(t, func() bool {
		for _, task := range tasks {
			if task.filePath == modelPath {
				return true
			}
		}
		return false
	}, "model.moc should be scheduled for download")
}
