package model_test

import (
	"sort"
	"testing"

	"github.com/A-kirami/bestdori-live2d-downloader/pkg/model"
	"github.com/stretchr/testify/require"
)

func TestCostumeLessTreatsBiliPrefixAsSameCharaSeries(t *testing.T) {
	costumes := []model.Live2dAsset{
		{Server: "jp", Costume: "016_cafe"},
		{Server: "cn", Costume: "bili_016_collabo_ssr"},
		{Server: "jp", Costume: "016_live_event_08_ssr"},
		{Server: "jp", Costume: "016_delta"},
	}

	sort.Slice(costumes, func(i, j int) bool {
		return model.CostumeLess(costumes[i], costumes[j])
	})

	require.Equal(t, []string{
		"016_cafe",
		"bili_016_collabo_ssr",
		"016_delta",
		"016_live_event_08_ssr",
	}, []string{
		costumes[0].Costume,
		costumes[1].Costume,
		costumes[2].Costume,
		costumes[3].Costume,
	})
}
