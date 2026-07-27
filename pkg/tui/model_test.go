package tui_test

import (
	"testing"

	"github.com/A-kirami/bestdori-live2d-downloader/pkg/model"
	"github.com/A-kirami/bestdori-live2d-downloader/pkg/tui"
	"github.com/stretchr/testify/require"
)

func TestUpdateListMsgSetsCharacterContext(t *testing.T) {
	t.Parallel()

	m := tui.NewModel()
	m.ErrorMessage = "previous error"
	m.Update(tui.UpdateListMsg{
		Items:          []*model.Live2dAsset{{Server: "jp", Costume: "001_casual"}},
		CharaID:        1,
		CharaName:      "香澄",
		ExtraCharaName: "户山香澄",
	})

	require.Equal(t, tui.StateList, m.State)
	require.Equal(t, 1, m.CurrentCharaID)
	require.Equal(t, "香澄", m.CurrentCharaName)
	require.Equal(t, "户山香澄", m.ExtraCharaName)
	require.Empty(t, m.ErrorMessage)
}

func TestShowCharaListErrorMsg(t *testing.T) {
	t.Parallel()

	m := tui.NewModel()
	m.Update(tui.ShowCharaListErrorMsg{Message: "加载失败"})

	require.Equal(t, tui.StateCharaList, m.State)
	require.Equal(t, "加载失败", m.ErrorMessage)
}
