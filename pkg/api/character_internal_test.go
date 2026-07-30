package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/A-kirami/bestdori-live2d-downloader/pkg/model"
	"github.com/stretchr/testify/require"
)

func TestResolveCharaColor(t *testing.T) {
	require.Equal(t, "#123456", resolveCharaColor(1, " #123456 "))
	require.Equal(t, "#DD33CC", resolveCharaColor(601, ""))
	require.Equal(t, fallbackCharaColor, resolveCharaColor(225, ""))
}

func TestParseBandInfoList(t *testing.T) {
	bands := parseBandInfoList(map[string]any{
		"1":       map[string]any{"bandName": []any{"Poppin'Party", nil, nil, " Poppin'Party\x1b "}},
		"2":       map[string]any{"bandName": []any{"Afterglow"}},
		"invalid": map[string]any{"bandName": []any{"Ignored"}},
		"3":       map[string]any{"bandName": "not-a-list"},
	})

	require.Equal(t, []model.BandInfo{
		{ID: 1, Name: "Poppin'Party"},
		{ID: 2, Name: "Afterglow"},
	}, bands)
}

func TestParseBestdoriID(t *testing.T) {
	require.Equal(t, 45, parseBestdoriID(float64(45)))
	require.Zero(t, parseBestdoriID(float64(-1)))
	require.Zero(t, parseBestdoriID(1.5))
	require.Zero(t, parseBestdoriID(float64(1001)))
	require.Zero(t, parseBestdoriID("45"))
}

func TestGetCharacterInfoListParsesBandID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		if _, err := response.Write([]byte(`{
			"1":{"bandId":1,"characterName":["Kasumi",null,null,"户山香澄"],"colorCode":"#FF5522"},
			"2":{"bandId":1.5,"characterName":["Tae",null,null,"花园多惠"],"colorCode":"#0077DD"}
		}`)); err != nil {
			t.Errorf("write character response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	client := NewClient()
	client.charaRosterURL = server.URL
	client.useCharaCache = false
	characters, err := client.GetCharacterInfoList(context.Background())

	require.NoError(t, err)
	require.Len(t, characters, 2)
	require.Equal(t, 1, characters[0].BandID)
	require.Zero(t, characters[1].BandID)
}
