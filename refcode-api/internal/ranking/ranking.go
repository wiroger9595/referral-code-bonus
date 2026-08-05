// Package ranking 決定服務商頁面上推薦碼的排列順序。
//
// Phase 1 只有自然區：加權隨機輪播。Phase 3 會在前面插入競價區
// （最多 3 個，effective_bid = bid × 品質因子），自然區邏輯不變。
package ranking

import (
	"cmp"
	"math"
	"math/rand/v2"
	"slices"
	"time"

	"github.com/google/uuid"
)

// Candidate 是排序需要的最小資訊，跟 DB row 解耦，方便測試。
type Candidate struct {
	ID           uuid.UUID
	QualityScore int32
	Impressions  int64
	CreatedAt    time.Time
	// 上架者是不是 Pro——這是付費賣點之一（paywall 上寫「優先曝光」），
	// 不是排序演算法自己要的概念，純粹為了兌現那個賣點才加進來。
	IsPro bool
}

type Params struct {
	// 新碼的曝光加成上限與衰減半衰期。
	FreshnessBoost   float64
	FreshnessHalfDay float64
	// 曝光懲罰的軟上限：累積這麼多次曝光時權重約打對折。
	ImpressionSoftCap float64
	// Pro 上架者的加權比例，例如 0.15 代表 +15%。故意設得比新鮮度加成小很多——
	// 這是加分不是分區，爛碼不該因為 Pro 就贏過品質好的免費碼太多。
	ProBoost float64
}

func DefaultParams() Params {
	return Params{
		FreshnessBoost:    0.5,
		FreshnessHalfDay:  7,
		ImpressionSoftCap: 100,
		ProBoost:          0.15,
	}
}

// Weight 是單一候選的權重。四個因子相乘：
//
//	品質分數   —— 回報數據決定，爛碼自然沉下去
//	新鮮度加成 —— 剛上架的碼給一段時間的加成，否則新人永遠排不上
//	曝光懲罰   —— 已經拿到很多曝光的碼權重下降，把機會讓出來
//	Pro 加成   —— 兌現 paywall 上的「優先曝光」賣點，幅度刻意壓低
//
// 第三項是這套機制的重點：沒有它，前幾名會被同一批碼長期佔據，
// 新上架的人看不到成效就不會再回來上架，供給端會枯竭。
func (p Params) Weight(c Candidate, now time.Time) float64 {
	quality := float64(c.QualityScore)
	if quality < 1 {
		quality = 1 // 權重為 0 會讓候選永遠抽不中，保留最低限度的翻身機會
	}

	ageDays := now.Sub(c.CreatedAt).Hours() / 24
	if ageDays < 0 {
		ageDays = 0
	}
	freshness := 1 + p.FreshnessBoost*math.Exp(-ageDays/p.FreshnessHalfDay)

	exposure := 1 / (1 + float64(c.Impressions)/p.ImpressionSoftCap)

	pro := 1.0
	if c.IsPro {
		pro = 1 + p.ProBoost
	}

	return quality * freshness * exposure * pro
}

// Rank 依權重做無放回加權抽樣，回傳打散後的順序。
//
// 用 Efraimidis-Spirakis：每個候選抽 key = u^(1/w)，取 key 最大的前 k 個。
// 這比「按權重逐次抽出再移除」快，且結果分布等價。
func Rank(candidates []Candidate, p Params, now time.Time, rng *rand.Rand) []Candidate {
	type keyed struct {
		c   Candidate
		key float64
	}

	keys := make([]keyed, len(candidates))
	for i, c := range candidates {
		u := rng.Float64()
		if u <= 0 {
			u = math.SmallestNonzeroFloat64
		}
		keys[i] = keyed{c: c, key: math.Pow(u, 1/p.Weight(c, now))}
	}

	// key 由大到小就是抽樣順序。
	slices.SortFunc(keys, func(a, b keyed) int {
		return cmp.Compare(b.key, a.key)
	})

	out := make([]Candidate, len(keys))
	for i, k := range keys {
		out[i] = k.c
	}
	return out
}
