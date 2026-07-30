package tui_test

import (
	"testing"

	"github.com/A-kirami/bestdori-live2d-downloader/pkg/config"
	"github.com/A-kirami/bestdori-live2d-downloader/pkg/model"
	"github.com/A-kirami/bestdori-live2d-downloader/pkg/tui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"
)

func TestUpdateListMsgSetsCharacterNames(t *testing.T) {
	t.Parallel()

	m := tui.NewModel()
	m.ErrorMessage = "previous error"
	m.Update(tui.UpdateListMsg{
		Items:          []*model.Live2dAsset{{Server: "jp", Costume: "001_casual"}},
		CharaName:      "香澄",
		ExtraCharaName: "户山香澄",
	})

	require.Equal(t, tui.StateList, m.State)
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

func TestCharacterLoadResultIsIgnoredAfterReturningToCharacterList(t *testing.T) {
	t.Parallel()

	m := tui.NewModel()
	m.Update(tui.UpdateCharaListMsg{Characters: []model.CharacterInfo{{ID: 1, Name: "户山香澄"}}})
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	m.Update(tui.UpdateListMsg{
		Items:     []*model.Live2dAsset{{Server: "jp", Costume: "001_casual"}},
		CharaName: "香澄",
	})
	m.Update(tui.ShowCharaListErrorMsg{Message: "旧请求失败"})

	require.Equal(t, tui.StateCharaList, m.State)
	require.Empty(t, m.ErrorMessage)
}

func TestCharacterLoadResultMustMatchLatestSelection(t *testing.T) {
	t.Parallel()

	m := tui.NewModel()
	m.Update(tui.UpdateCharaListMsg{Characters: []model.CharacterInfo{
		{ID: 1, Name: "户山香澄"},
		{ID: 2, Name: "花园多惠"},
	}})

	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	firstRequest := <-m.GetCharaSelectChan()
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	secondRequest := <-m.GetCharaSelectChan()

	m.Update(tui.UpdateListMsg{
		Items:     []*model.Live2dAsset{{Server: "jp", Costume: "001_casual"}},
		CharaName: "香澄",
		RequestID: firstRequest.RequestID,
	})
	m.Update(tui.ShowCharaListErrorMsg{Message: "旧请求失败", RequestID: firstRequest.RequestID})

	require.Equal(t, tui.StateLoading, m.State)
	require.Empty(t, m.CurrentCharaName)
	require.Empty(t, m.ErrorMessage)

	m.Update(tui.UpdateListMsg{
		Items:     []*model.Live2dAsset{{Server: "jp", Costume: "002_casual"}},
		CharaName: "多惠",
		RequestID: secondRequest.RequestID,
	})

	require.Equal(t, tui.StateList, m.State)
	require.Equal(t, "多惠", m.CurrentCharaName)
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

func TestSelectAllTogglesEveryItem(t *testing.T) {
	t.Parallel()

	m := tui.NewModel()
	m.Update(tui.UpdateListMsg{
		Items: []*model.Live2dAsset{
			{Server: "jp", Costume: "001_casual"},
			{Server: "jp", Costume: "001_school"},
		},
	})

	m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	require.Len(t, m.GetSelectedItems(), 2)

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	require.Empty(t, m.GetSelectedItems())
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
	request := <-m.GetCharaSelectChan()
	require.Equal(t, 1, request.CharaID)
	require.NotZero(t, request.RequestID)
}

func TestNewModelUsesConfiguredNamingMode(t *testing.T) {
	config.Init()
	t.Cleanup(config.Init)
	config.Get().NamingMode = config.NamingModeOriginal

	m := tui.NewModel()

	require.Equal(t, config.NamingModeOriginal, m.NamingMode)
}
