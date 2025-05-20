// Package utils 提供了各种工具函数
// 包括字符串处理、角色搜索等功能
package utils

import (
	"strconv"
	"strings"

	"slices"

	"github.com/adrg/strutil/metrics"
)

// compareSimilarity 比较两个相似度并决定是否更新最佳匹配
// 参数:
//   - searchName: 搜索名称
//   - searchCandidate: 候选名称
//   - candidateParts: 候选名称分词列表
//   - searchParts: 搜索名称分词列表
//   - candidate: 当前候选名称
//   - bestMatch: 当前最佳匹配
//   - id: 当前候选ID
//   - bestID: 当前最佳匹配ID
//
// 返回:
//   - string: 更新后的最佳匹配名称
//   - string: 更新后的最佳匹配ID
//   - bool: 是否需要更新最佳匹配
func compareSimilarity(
	searchName string,
	searchCandidate string,
	candidateParts []string,
	searchParts []string,
	candidate string,
	bestMatch string,
	id string,
	bestID string,
) (string, string, bool) {
	// 1. 优先选择完全匹配
	if searchName == searchCandidate {
		return candidate, id, true
	}

	// 2. 优先选择名字部分完全匹配的
	nameMatched := false
	for _, namePart := range searchParts {
		if slices.Contains(candidateParts, namePart) {
			nameMatched = true
			break
		}
	}
	if nameMatched {
		return candidate, id, true
	}

	// 3. 优先选择更短的匹配（因为通常名字越短越可能是昵称或简称）
	if len(candidate) < len(bestMatch) {
		return candidate, id, true
	}

	// 4. 如果长度相同，优先选择 ID 较小的
	if len(candidate) == len(bestMatch) {
		if id < bestID {
			return candidate, id, true
		}
	}

	return bestMatch, bestID, false
}

// calculateSimilarity 计算两个字符串之间的相似度
// 参数:
//   - swg: Smith-Waterman-Gotoh 算法实例
//   - searchName: 搜索名称
//   - searchCandidate: 候选名称
//   - searchParts: 搜索名称分词列表
//   - candidateParts: 候选名称分词列表
//
// 返回:
//   - float64: 相似度（0-1之间）
func calculateSimilarity(
	swg *metrics.SmithWatermanGotoh,
	searchName, searchCandidate string,
	searchParts, candidateParts []string,
) float64 {
	// 计算基础相似度
	sim := swg.Compare(searchName, searchCandidate)

	// 检查是否是完全匹配
	if searchName == searchCandidate {
		return 1.0
	}

	// 检查名字部分匹配
	for _, namePart := range searchParts {
		if slices.Contains(candidateParts, namePart) {
			sim += 0.3 // 给予部分匹配额外的权重
		}
	}

	return sim
}

// isValidCandidate 检查候选ID是否有效
// 参数:
//   - id: 候选ID
//
// 返回:
//   - bool: ID是否有效
func isValidCandidate(id string) bool {
	if idNum, err := strconv.Atoi(id); err != nil || idNum > 1000 {
		return false
	}
	return true
}

// FindBestMatch 使用 Smith-Waterman-Gotoh 算法找到最佳匹配
// 该算法用于在角色名称列表中查找与输入名称最匹配的角色
// 参数:
//   - name: 要搜索的名称
//   - candidates: 候选名称映射，key 为角色ID，value 为角色名称列表
//
// 返回:
//   - string: 最佳匹配的角色ID
//   - string: 最佳匹配的角色名称
//   - float64: 匹配相似度（0-1之间）
//
// 算法说明:
// 1. 使用 Smith-Waterman-Gotoh 算法计算基础相似度
// 2. 对完全匹配的情况给予最高权重
// 3. 对部分匹配的情况给予额外权重
// 4. 在相似度相同的情况下，使用以下规则：
//   - 优先选择完全匹配
//   - 优先选择名字部分完全匹配的
//   - 优先选择更短的匹配（通常更可能是昵称或简称）
//   - 如果长度相同，优先选择 ID 较小的
func FindBestMatch(name string, candidates map[string][]string) (string, string, float64) {
	var maxSimilarity float64
	var bestMatch string
	var bestID string

	// 初始化 Smith-Waterman-Gotoh 算法
	swg := metrics.NewSmithWatermanGotoh()
	swg.CaseSensitive = false
	swg.GapPenalty = -0.1
	swg.Substitution = metrics.MatchMismatch{
		Match:    1,
		Mismatch: -0.5,
	}

	// 预处理输入名称
	searchName := strings.TrimSpace(strings.ToLower(name))
	searchParts := strings.Fields(searchName)

	for id, names := range candidates {
		if !isValidCandidate(id) {
			continue
		}

		for _, candidate := range names {
			if candidate == "" {
				continue
			}

			// 预处理候选名称
			searchCandidate := strings.TrimSpace(strings.ToLower(candidate))
			candidateParts := strings.Fields(searchCandidate)

			// 计算相似度
			sim := calculateSimilarity(swg, searchName, searchCandidate, searchParts, candidateParts)

			// 如果相似度更高，直接更新
			if sim > maxSimilarity {
				maxSimilarity = sim
				bestMatch = candidate
				bestID = id
				continue
			}

			// 如果相似度相同，使用额外的规则来决定
			if sim == maxSimilarity {
				newBestMatch, newBestID, shouldUpdate := compareSimilarity(
					searchName,
					searchCandidate,
					candidateParts,
					searchParts,
					candidate,
					bestMatch,
					id,
					bestID,
				)
				if shouldUpdate {
					bestMatch = newBestMatch
					bestID = newBestID
				}
			}
		}
	}

	return bestID, bestMatch, maxSimilarity
}
