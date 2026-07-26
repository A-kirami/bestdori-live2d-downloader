package naming

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

func translateDatedCostume(suffix string, eventNames map[int]string) string {
	if matches := regexp.MustCompile(`^(\d{4})af$`).FindStringSubmatch(suffix); len(matches) > 1 {
		return matches[1] + "愚人节"
	}

	if matches := regexp.MustCompile(`^(.+)-(\d{4})-(.+)$`).FindStringSubmatch(suffix); len(matches) > 3 {
		baseName := TranslateCostumeSuffix(matches[1], eventNames)
		if baseName != "" {
			variantName := translateVariant(matches[3])
			return fmt.Sprintf("%s(%s)(%s)", baseName, matches[2], variantName)
		}
	}

	if matches := regexp.MustCompile(`^(.+)-(\d{4})$`).FindStringSubmatch(suffix); len(matches) > 2 {
		baseName := TranslateCostumeSuffix(matches[1], eventNames)
		if baseName != "" {
			return fmt.Sprintf("%s(%s)", baseName, matches[2])
		}
	}

	if matches := regexp.MustCompile(`^(\d{4})_(.+)$`).FindStringSubmatch(suffix); len(matches) > 2 {
		baseName := TranslateCostumeSuffix(matches[2], eventNames)
		if baseName != "" {
			return fmt.Sprintf("%s(%s)", baseName, matches[1])
		}
	}

	if matches := regexp.MustCompile(`^chapter(\d+)_(.+)$`).FindStringSubmatch(suffix); len(matches) > 2 {
		chapterNum := matches[1]
		costumeType := matches[2]
		var typeName string
		switch costumeType {
		case "live":
			typeName = "Live"
		case costumePajamas:
			typeName = "睡衣"
		default:
			typeName = costumeType
		}
		if chapterNum == "0" {
			return "序章" + typeName
		}
		return fmt.Sprintf("第%s章%s", chapterNum, typeName)
	}

	return ""
}

func translateEventCostume(suffix string, eventNames map[int]string) string {
	if matches := regexp.MustCompile(`^event_?(\d+)$`).FindStringSubmatch(suffix); len(matches) > 1 {
		eventID, err := strconv.Atoi(matches[1])
		if err == nil {
			if eventName, ok := eventNames[eventID]; ok {
				return eventName
			}
			return fmt.Sprintf("活动%s", matches[1])
		}
	}

	if matches := regexp.MustCompile(`^event_?(\d+)_story_?(\w+)$`).FindStringSubmatch(suffix); len(matches) > 2 {
		eventID, err := strconv.Atoi(matches[1])
		if err == nil {
			if eventName, ok := eventNames[eventID]; ok {
				return fmt.Sprintf("%s(剧情%s)", eventName, matches[2])
			}
			return fmt.Sprintf("活动%s(剧情%s)", matches[1], matches[2])
		}
	}

	if matches := regexp.MustCompile(`^live_event_(\d+)_(r|sr|ssr|ur)$`).FindStringSubmatch(suffix); len(matches) > 2 {
		eventID, err := strconv.Atoi(matches[1])
		if err == nil {
			if eventName, ok := eventNames[eventID]; ok {
				return fmt.Sprintf("%s(%s)", eventName, strings.ToUpper(matches[2]))
			}
			return fmt.Sprintf("活动%s(%s)", matches[1], strings.ToUpper(matches[2]))
		}
	}

	return ""
}

func translateCategoryCostume(suffix string) (string, bool) {
	if strings.HasPrefix(suffix, "collabo") {
		return "联动服", true
	}
	if strings.HasPrefix(suffix, "dream_festival") {
		return "梦想祭", true
	}
	if strings.HasPrefix(suffix, "birthday") {
		return "生日限定", true
	}

	if strings.Contains(suffix, "general_election") {
		if matches := regexp.MustCompile(`(\d+).*general_election`).FindStringSubmatch(suffix); len(matches) > 1 {
			return fmt.Sprintf("第%s届总选举", matches[1]), true
		}
		return "总选举", true
	}

	return translateStoryCostume(suffix)
}

func translateStoryCostume(suffix string) (string, bool) {
	if matches := regexp.MustCompile(`^band_story_(\d+)$`).FindStringSubmatch(suffix); len(matches) > 1 {
		return fmt.Sprintf("乐队故事%s", matches[1]), true
	}

	if matches := regexp.MustCompile(`^story_(\d+)(-.*)?$`).FindStringSubmatch(suffix); len(matches) > 1 {
		if matches[1] == "01" || matches[1] == "1" {
			return costumePractice, true
		}
		if matches[1] == "03" {
			return "米歇尔玩偶服", true
		}
		return "", true
	}

	if strings.HasPrefix(suffix, "miku_") {
		mikuSongs := map[string]string{
			"miku_shinkai":      "深海少女",
			"miku_migikata":     "右肩之蝶",
			"miku_rettou":       "左肩之蝶",
			"miku_alien":        "Alien Alien",
			"miku_lostone":      "Lost One的号哭",
			"miku_romecin":      "罗密欧与辛德瑞拉",
			"miku_nocturnality": "夜之蝶",
		}
		if name, ok := mikuSongs[suffix]; ok {
			return "初音联动·" + name, true
		}
		return "初音联动", true
	}

	if matches := regexp.MustCompile(`other-(\d+)$`).FindStringSubmatch(suffix); len(matches) > 1 {
		if matches[1] == "12" {
			return "活动小丑", true
		}
		if matches[1] == "41" {
			return "愚人节", true
		}
		return fmt.Sprintf("角色专属%s", matches[1]), true
	}

	return "", false
}
func translateBaseCostume(suffix string) string {
	baseCostumes := map[string]string{
		"casual":      "常服",
		"school":      "校服",
		"pajamas":     costumePajamas,
		"swimsuit":    "泳装",
		"yukata":      "浴衣",
		"halloween":   costumeHalloween,
		"christmas":   "圣诞",
		"furisode":    "振袖",
		"arbeit":      "打工服",
		"expose":      "《EXPOSE》演出服",
		"fantasy":     "Neo Fantasy Online",
		"delta":       "Delta变体服",
		"miko":        "巫女服",
		"apron":       "围裙",
		"store":       "店员服",
		"tracksuit":   "运动服",
		"garupa":      "Garupa",
		"popipa":      "Popipa",
		"anime":       "动画",
		"michelle":    "米歇尔(兔子玩偶服)",
		"ranger":      "游侠服",
		"cafe":        "咖啡厅",
		"fast_food":   "快餐",
		"gym":         "体操",
		"chairperson": "学生会长",
		"memorial":    "纪念",
		"stage":       "舞台",
		"practice":    "练习",
		"live":        "Live",
		"fes":         "祭典",
		"boss":        "Boss服",
		"robot":       "机器人服",
		"chispa":      "CHiSPA乐队服",
		"sumimi":      "Sumimi企划服",
		"nfo":         "《NFO》游戏服装",
		"wd":          "白色情人节",
	}

	for baseKey, baseName := range baseCostumes {
		if strings.Contains(suffix, baseKey) {
			return baseName
		}
	}
	return ""
}
