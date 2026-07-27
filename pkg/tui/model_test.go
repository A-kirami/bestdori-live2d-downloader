package tui_test

import (
	"testing"

	"github.com/A-kirami/bestdori-live2d-downloader/pkg/config"
	"github.com/A-kirami/bestdori-live2d-downloader/pkg/model"
	"github.com/A-kirami/bestdori-live2d-downloader/pkg/tui"
	tea "github.com/charmbracelet/bubbletea"
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

func TestFilteringPreservesSelections(t *testing.T) {
	t.Parallel()

	m := tui.NewModel()
	m.Update(tui.UpdateListMsg{
		Items: []*model.Live2dAsset{
			{Server: "jp", Costume: "001_casual"},
			{Server: "jp", Costume: "001_school"},
		},
		CostumeNames: map[string]string{
			"001_casual": "常服",
			"001_school": "校服",
		},
	})

	m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("校服")})
	require.Len(t, m.Live2dList.Items(), 1)
	m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	require.Len(t, m.Live2dList.Items(), 2)

	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	selected := <-m.GetSelectChan()
	require.Len(t, selected, 2)
}

func TestNamingModeChangesSelectedItemDisplayName(t *testing.T) {
	t.Parallel()

	m := tui.NewModel()
	m.Update(tui.UpdateListMsg{
		Items:        []*model.Live2dAsset{{Server: "jp", Costume: "001_casual"}},
		CostumeNames: map[string]string{"001_casual": "常服"},
	})
	m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	selected := <-m.GetSelectChan()
	require.Len(t, selected, 1)
	require.Equal(t, "001_casual (jp)", selected[0].DisplayName)
	require.Equal(t, config.NamingModeOriginal, selected[0].NamingMode)
}

func TestCharacterSelectionEntersLoadingState(t *testing.T) {
	t.Parallel()

	m := tui.NewModel()
	m.Update(tui.UpdateCharaListMsg{Characters: []model.CharacterInfo{{ID: 1, Name: "户山香澄"}}})

	require.Equal(t, tui.StateCharaList, m.State)
	require.Len(t, m.CharaList.Items(), 1)
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	require.Equal(t, tui.StateLoading, m.State)
	require.Equal(t, 1, <-m.GetCharaSelectChan())
}

func TestNewModelUsesConfiguredNamingMode(t *testing.T) {
	config.Init()
	t.Cleanup(config.Init)
	config.Get().NamingMode = config.NamingModeOriginal

	m := tui.NewModel()

	require.Equal(t, config.NamingModeOriginal, m.NamingMode)
}
