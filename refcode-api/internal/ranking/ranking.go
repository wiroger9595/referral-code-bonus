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
	// 開始曝光的時間，不是建立時間——碼在審核佇列排隊的期間還沒被任何人看到，
	// 那段時間不該計入年齡，否則審核慢一點就把黃金期整段吃掉。
	ListedAt time.Time
	// 上架者是不是 Pro。這是付費賣點，不是排序演算法自己要的概念。
	IsPro bool
}

type Params struct {
	// 新碼的曝光加成上限。
	FreshnessBoost float64

	// 黃金期：上架後這段時數內維持滿額加成，之後才開始衰減。
	// 衰減時數是時間常數——每過這麼久，剩下的加成掉到約 37%（1/e）。
	//
	// 免費與 Pro 走兩條完全不同的曲線，這是訂閱最主要的賣點：
	// 免費碼的黃金期只有幾小時，過了就迅速退回基準線，靠品質和曝光懲罰決定位置；
	// Pro 的碼則能在前排待上好幾天。差別在「待多久」而不是「能不能上去」——
	// 免費的新碼一樣拿得到滿額加成，只是留不住。
	FreeGraceHours float64
	FreeDecayHours float64
	ProGraceHours  float64
	ProDecayHours  float64

	// 曝光懲罰的軟上限：累積這麼多次曝光時權重約打對折。
	ImpressionSoftCap float64
	// Pro 上架者的常駐加權比例，例如 0.15 代表 +15%。
	//
	// 預設 0：Pro 的優勢刻意全部集中在上面那段黃金期，碼一旦過了新鮮期，
	// 位置就純粹由品質分數和曝光懲罰決定。付費能買到的是「起跑階段久一點」，
	// 買不到「永遠比別人前面」——後者會讓列表長期被付費者的老碼佔住，
	// 那正是曝光懲罰要防的事。要讓 Pro 的老碼也有一點優勢再調高這個值。
	ProBoost float64
}

func DefaultParams() Params {
	return Params{
		FreshnessBoost: 0.5,
		// 免費：6 小時黃金期，之後 3 小時掉到 37%，半天內就幾乎回到基準線。
		FreeGraceHours: 6,
		FreeDecayHours: 3,
		// Pro：3 天黃金期，之後用 7 天的時間常數慢慢退，跟改版前的曲線接近。
		ProGraceHours:     72,
		ProDecayHours:     168,
		ImpressionSoftCap: 100,
		ProBoost:          0,
	}
}

// Weight 是單一候選的權重。四個因子相乘：
//
//	品質分數   —— 回報數據決定，爛碼自然沉下去
//	新鮮度加成 —— 剛上架的碼給一段黃金期，否則新人永遠排不上；免費過期後迅速歸零
//	曝光懲罰   —— 已經拿到很多曝光的碼權重下降，把機會讓出來
//	Pro 加成   —— 常駐加分，預設關閉；Pro 的差別在黃金期長度，不在這裡
//
// 第三項是這套機制的重點：沒有它，前幾名會被同一批碼長期佔據，
// 新上架的人看不到成效就不會再回來上架，供給端會枯竭。
func (p Params) Weight(c Candidate, now time.Time) float64 {
	quality := float64(c.QualityScore)
	if quality < 1 {
		quality = 1 // 權重為 0 會讓候選永遠抽不中，保留最低限度的翻身機會
	}

	freshness := 1 + p.FreshnessBoost*p.freshness(c, now)

	exposure := 1 / (1 + float64(c.Impressions)/p.ImpressionSoftCap)

	pro := 1.0
	if c.IsPro {
		pro = 1 + p.ProBoost
	}

	return quality * freshness * exposure * pro
}

// freshness 回傳 0~1 的新鮮度殘量：黃金期內是滿的 1，之後指數衰減。
//
// 分段而不是單純指數，是為了讓「黃金期」這件事在產品上講得清楚——
// 免費用戶拿到的是明確的一段時間，不是一條說不出邊界在哪的曲線。
func (p Params) freshness(c Candidate, now time.Time) float64 {
	grace, decay := p.FreeGraceHours, p.FreeDecayHours
	if c.IsPro {
		grace, decay = p.ProGraceHours, p.ProDecayHours
	}

	// 時間往前跳（時鐘校正、或資料把 ListedAt 寫成未來）時當作剛上架，
	// 不要讓負數年齡把加成放大到超過上限。
	ageHours := now.Sub(c.ListedAt).Hours()
	if ageHours <= grace {
		return 1
	}
	if decay <= 0 {
		return 0 // 衰減時數設 0 代表黃金期一過就沒有加成
	}
	return math.Exp(-(ageHours - grace) / decay)
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
