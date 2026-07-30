// Package naming 提供面向用户的 Live2D 命名规则.
package naming

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const (
	costumePajamas   = "睡衣"
	costumeHalloween = "万圣节"
	costumePractice  = "剧情初始服装"
)

func translateVariant(variant string) string {
	variants := map[string]string{
		"penlight": "荧光棒",
		"nocap":    "无帽",
		"sunglass": "墨镜",
	}
	if name, ok := variants[variant]; ok {
		return name
	}
	return variant
}

// TranslateCostumeSuffixWithStoryCount 根据剧情数量翻译服装后缀.
//
//nolint:gocognit,nestif // 剧情翻译规则本身包含多个格式分支.
func TranslateCostumeSuffixWithStoryCount(
	suffix string,
	eventNames map[int]string,
	charaEventStoryCounts map[string]int,
	charaID string,
) string {
	if matches := regexp.MustCompile(`^event_?(\d+)_story_?(\w+)$`).FindStringSubmatch(suffix); len(matches) > 2 {
		eventID, err := strconv.Atoi(matches[1])
		if err == nil {
			storyNum := strings.TrimLeft(matches[2], "0")
			if storyNum == "" {
				storyNum = "1"
			}
			key := charaID + ":" + matches[1]
			if charaEventStoryCounts[key] > 1 {
				if eventName, ok := eventNames[eventID]; ok {
					return fmt.Sprintf("%s·剧情%s", eventName, storyNum)
				}
				return fmt.Sprintf("活动%s·剧情%s", matches[1], storyNum)
			}
			if eventName, ok := eventNames[eventID]; ok {
				return fmt.Sprintf("%s·剧情", eventName)
			}
			return fmt.Sprintf("活动%s·剧情", matches[1])
		}
	}

	if matches := regexp.MustCompile(`^event_?(\d+)_story$`).FindStringSubmatch(suffix); len(matches) > 1 {
		eventID, err := strconv.Atoi(matches[1])
		if err == nil {
			key := charaID + ":" + matches[1]
			if charaEventStoryCounts[key] > 1 {
				if eventName, ok := eventNames[eventID]; ok {
					return fmt.Sprintf("%s·剧情1", eventName)
				}
				return fmt.Sprintf("活动%s·剧情1", matches[1])
			}
			if eventName, ok := eventNames[eventID]; ok {
				return fmt.Sprintf("%s·剧情", eventName)
			}
			return fmt.Sprintf("活动%s·剧情", matches[1])
		}
	}

	return TranslateCostumeSuffix(suffix, eventNames)
}

// TranslateCostumeSuffix 使用精确和模式规则翻译服装后缀.
//

func TranslateCostumeSuffix(suffix string, eventNames map[int]string) string {
	exactMatches := map[string]string{
		"casual":                    "常服",
		"casual_summer":             "夏季常服",
		"casual_winter":             "冬季常服",
		"casual_winter-sunglass":    "常服(冬·墨镜)",
		"casual_v3":                 "常服v3",
		"school":                    "校服",
		"school_winter":             "冬服",
		"school_winter_2":           "冬服2",
		"school_winter_s2":          "冬服s2",
		"school_winter_v3":          "冬服v3",
		"school_summer":             "夏服",
		"school_summer_s2":          "夏服s2",
		"school_summer_v3":          "夏服v3",
		"school_armband":            "校服(臂章)",
		"jh_school_winter":          "初中冬服",
		"uniform":                   "制服",
		"default":                   "默认",
		"general":                   "通用",
		"gym_clothes":               "体操服",
		"tracksuit":                 "运动服",
		"pajamas":                   costumePajamas,
		"swimsuit":                  "泳装",
		"swim_swit":                 "泳装",
		"apron":                     "围裙",
		"cafe":                      "咖啡厅制服",
		"fast_food":                 "快餐店制服",
		"arbeit":                    "打工服",
		"store":                     "店员服",
		"chairperson_casual":        "学生会长服",
		"practice_clothes":          costumePractice,
		"stage_costume":             "舞台服装",
		"wd_practice":               "白色情人节练习服",
		"garupa_t":                  "Garupa T恤",
		"memorial_middle_school":    "纪念中学制服",
		"af":                        "愚人节",
		"xmas":                      "圣诞",
		"hw":                        costumeHalloween,
		"halloween":                 costumeHalloween,
		"halloween_without_lantern": "万圣节(无灯)",
		"furisode":                  "振袖",
		"yukata":                    "浴衣",
		"anniv":                     "周年纪念",
		"special_5th":               "5周年纪念",
		"girlparty2019":             "女子聚会2019",
		"precious_summer":           "珍贵夏日",
		"anime_live":                "动画Live",
		"popipa_fes":                "Popipa祭典",
		"kirameki_festival":         "闪光祭典",
		"kirameki_festival_coat":    "闪光祭典(外套)",
		"chapter0_live":             "序章Live",
		"chapter0_pajamas":          "序章睡衣",
		"romeo":                     "罗密欧",
		"juliet":                    "朱丽叶",
		"michelle":                  "米歇尔(兔子玩偶服)",
		"michelle_ranger":           "米歇尔·游侠(兔子玩偶服)",
		"miko":                      "巫女服",
		"fantasy":                   "Neo Fantasy Online",
		"fantasy_01":                "Neo Fantasy Online 01",
		"delta":                     "Delta变体服",
		"expose":                    "《EXPOSE》演出服",
		"ranger":                    "游侠服",
		"chispa":                    "CHiSPA乐队服",
		"sumimi":                    "Sumimi企划服",
		"nfo01":                     "《NFO》游戏服装01",
		"boss":                      "Boss服",
		"robot":                     "机器人服",
		"live_default":              "初始打歌服",
		"live_practice":             costumePractice,
		"live_sr_01":                "Live SR",
		"live_ssr_01":               "Live SSR",
		"vocal_limited_sr":          "Vocal限定SR",
		"vocal_limited_ssr":         "Vocal限定SSR",
	}

	if name, ok := exactMatches[suffix]; ok {
		return name
	}

	if name := translateDatedCostume(suffix, eventNames); name != "" {
		return name
	}
	if name := translateEventCostume(suffix, eventNames); name != "" {
		return name
	}
	if name, matched := translateCategoryCostume(suffix); matched {
		return name
	}

	return translateBaseCostume(suffix)
}
