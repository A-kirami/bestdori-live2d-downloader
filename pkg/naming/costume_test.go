package naming_test

import (
	"testing"

	"github.com/A-kirami/bestdori-live2d-downloader/pkg/naming"
	"github.com/stretchr/testify/require"
)

func TestTranslateCostumeSuffix(t *testing.T) {
	eventNames := map[int]string{12: "测试活动"}
	tests := []struct {
		name   string
		suffix string
		want   string
	}{
		{name: "exact", suffix: "casual", want: "常服"},
		{name: "year and variant", suffix: "casual-2023-penlight", want: "常服(2023)(荧光棒)"},
		{name: "year prefix", suffix: "2024_furisode", want: "振袖(2024)"},
		{name: "chapter", suffix: "chapter2_live", want: "第2章Live"},
		{name: "chapter pajamas", suffix: "chapter2_pajamas", want: "第2章睡衣"},
		{name: "event", suffix: "event_12", want: "测试活动"},
		{name: "event card", suffix: "live_event_12_ssr", want: "测试活动(SSR)"},
		{name: "story practice", suffix: "story_01", want: "剧情初始服装"},
		{name: "unmapped story", suffix: "story_02-live", want: ""},
		{name: "collaboration", suffix: "collabo_example", want: "联动服"},
		{name: "miku song", suffix: "miku_shinkai", want: "初音联动·深海少女"},
		{name: "other costume", suffix: "other-12", want: "活动小丑"},
		{name: "unknown", suffix: "unmapped_costume", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, naming.TranslateCostumeSuffix(tt.suffix, eventNames))
		})
	}
}

func TestTranslateCostumeSuffixWithStoryCount(t *testing.T) {
	eventNames := map[int]string{12: "测试活动"}

	require.Equal(t, "测试活动·剧情", naming.TranslateCostumeSuffixWithStoryCount(
		"event_12_story_02", eventNames, map[string]int{"1:12": 1}, "1",
	))
	require.Equal(t, "测试活动·剧情2", naming.TranslateCostumeSuffixWithStoryCount(
		"event_12_story_02", eventNames, map[string]int{"1:12": 2}, "1",
	))
}
