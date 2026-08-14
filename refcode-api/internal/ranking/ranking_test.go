package ranking

import (
	"math"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestWeightFavorsLowExposure(t *testing.T) {
	p := DefaultParams()
	now := time.Now()
	listed := now.Add(-30 * 24 * time.Hour) // 抽掉新鮮度加成的影響

	fresh := Candidate{QualityScore: 60, Impressions: 0, ListedAt: listed}
	saturated := Candidate{QualityScore: 60, Impressions: 1000, ListedAt: listed}

	if p.Weight(fresh, now) <= p.Weight(saturated, now) {
		t.Fatalf("曝光少的碼權重應該較高：fresh=%v saturated=%v",
			p.Weight(fresh, now), p.Weight(saturated, now))
	}
}

// 曝光懲罰是這套排序的重點：品質相同時，已經吃掉大量曝光的碼
// 必須把位置讓給還沒被看過的碼，否則新上架的人永遠排不進去。
func TestRankGivesNewcomersRealChance(t *testing.T) {
	now := time.Now()
	listed := now.Add(-30 * 24 * time.Hour)

	newcomer := Candidate{ID: uuid.New(), QualityScore: 60, Impressions: 0, ListedAt: listed}
	candidates := []Candidate{newcomer}
	for i := 0; i < 9; i++ {
		candidates = append(candidates, Candidate{
			ID: uuid.New(), QualityScore: 60, Impressions: 2000, ListedAt: listed,
		})
	}

	rng := rand.New(rand.NewPCG(1, 2))
	const runs = 2000
	firstPlace := 0
	for i := 0; i < runs; i++ {
		if Rank(candidates, DefaultParams(), now, rng)[0].ID == newcomer.ID {
			firstPlace++
		}
	}

	// 均分的話是 10%。權重差距約 21 倍，實測落在 65~75% 之間；
	// 這裡只驗「明顯高於均分」，避免把測試綁死在特定參數上。
	ratio := float64(firstPlace) / runs
	if ratio < 0.5 {
		t.Fatalf("新碼拿到第一名的比例過低：%.2f%%", ratio*100)
	}
}

func TestRankKeepsAllCandidates(t *testing.T) {
	now := time.Now()
	candidates := make([]Candidate, 50)
	for i := range candidates {
		candidates[i] = Candidate{ID: uuid.New(), QualityScore: int32(i + 1), ListedAt: now}
	}

	rng := rand.New(rand.NewPCG(3, 4))
	ranked := Rank(candidates, DefaultParams(), now, rng)

	if len(ranked) != len(candidates) {
		t.Fatalf("排序後數量不一致：%d != %d", len(ranked), len(candidates))
	}
	seen := map[uuid.UUID]bool{}
	for _, c := range ranked {
		if seen[c.ID] {
			t.Fatalf("排序結果有重複：%s", c.ID)
		}
		seen[c.ID] = true
	}
}

// Pro 加成要是真的加成，不是裝飾——付費賣的是曝光，新鮮期內的權重就得真的比同條件的免費碼高。
// 用 24 小時這個時點：免費的黃金期已經過完，Pro 的還沒。
func TestWeightFavorsPro(t *testing.T) {
	p := DefaultParams()
	now := time.Now()
	listed := now.Add(-24 * time.Hour)

	free := Candidate{QualityScore: 60, Impressions: 10, ListedAt: listed}
	pro := Candidate{QualityScore: 60, Impressions: 10, ListedAt: listed, IsPro: true}

	if p.Weight(pro, now) <= p.Weight(free, now) {
		t.Fatalf("Pro 候選權重應該較高：pro=%v free=%v", p.Weight(pro, now), p.Weight(free, now))
	}
}

// ProBoost 預設 0，代表兩邊的新鮮期都燒完之後權重必須完全相同。
// 付費買到的是起跑階段，不是永久的排序特權——這條守住的是免費用戶留下來的理由。
func TestProHasNoEdgeOnOldCodes(t *testing.T) {
	p := DefaultParams()
	now := time.Now()
	listed := now.Add(-365 * 24 * time.Hour) // 遠超過兩邊的黃金期與衰減尺度

	free := Candidate{QualityScore: 60, Impressions: 10, ListedAt: listed}
	pro := Candidate{QualityScore: 60, Impressions: 10, ListedAt: listed, IsPro: true}

	if diff := math.Abs(p.Weight(pro, now) - p.Weight(free, now)); diff > 1e-9 {
		t.Fatalf("老碼不該因為 Pro 而有優勢：pro=%v free=%v", p.Weight(pro, now), p.Weight(free, now))
	}
}

// 免費帳戶的黃金期一過，新鮮度加成要迅速消失——這是訂閱賣點成立的前提。
// 但「消失」指的是退回基準線，不是被罰到比老碼還低。
func TestFreeFreshnessCollapsesAfterGrace(t *testing.T) {
	p := DefaultParams()
	now := time.Now()

	within := Candidate{QualityScore: 60, Impressions: 10, ListedAt: now.Add(-5 * time.Hour)}
	justAfter := Candidate{QualityScore: 60, Impressions: 10, ListedAt: now.Add(-18 * time.Hour)}
	old := Candidate{QualityScore: 60, Impressions: 10, ListedAt: now.Add(-90 * 24 * time.Hour)}

	wWithin, wAfter, wOld := p.Weight(within, now), p.Weight(justAfter, now), p.Weight(old, now)

	// 黃金期內拿滿額加成，18 小時後應該已經掉掉大半。
	if wAfter > wWithin*0.8 {
		t.Fatalf("黃金期過後衰減太慢：within=%v after=%v", wWithin, wAfter)
	}
	if wAfter < wOld {
		t.Fatalf("衰減後不該低於基準線：after=%v old=%v", wAfter, wOld)
	}
}

// Pro 的差別在「待多久」：同樣是免費碼早就退回基準線的時間點，Pro 還在滿額黃金期。
func TestProKeepsFreshnessLongerThanFree(t *testing.T) {
	p := DefaultParams()
	now := time.Now()
	listed := now.Add(-24 * time.Hour) // 免費的 6 小時黃金期早就過了，Pro 的 72 小時還沒

	free := Candidate{QualityScore: 60, Impressions: 10, ListedAt: listed}
	pro := Candidate{QualityScore: 60, Impressions: 10, ListedAt: listed, IsPro: true}

	if got := p.freshness(pro, now); got != 1 {
		t.Fatalf("Pro 在 24 小時時仍應處於黃金期，殘量 = %v", got)
	}
	if got := p.freshness(free, now); got > 0.01 {
		t.Fatalf("免費在 24 小時時新鮮度應已耗盡，殘量 = %v", got)
	}

	// 差距完全來自新鮮度加成上限：1+FreshnessBoost 對 1，也就是 1.5 倍。
	want := 1 + p.FreshnessBoost
	if ratio := p.Weight(pro, now) / p.Weight(free, now); math.Abs(ratio-want) > 0.02 {
		t.Fatalf("Pro 對免費的權重比例 = %.3f，預期約 %.2f", ratio, want)
	}
}

// 上架時間若因時鐘校正落在未來，加成不能被放大到超過上限。
func TestFutureListedAtDoesNotAmplify(t *testing.T) {
	p := DefaultParams()
	now := time.Now()

	future := Candidate{QualityScore: 60, ListedAt: now.Add(48 * time.Hour)}
	justListed := Candidate{QualityScore: 60, ListedAt: now}

	if p.Weight(future, now) != p.Weight(justListed, now) {
		t.Fatalf("未來時間的候選權重應與剛上架相同：future=%v now=%v",
			p.Weight(future, now), p.Weight(justListed, now))
	}
}

// 品質分數為 0 的碼仍要留最低權重，否則一旦沉底就再也拿不到翻身的曝光。
func TestZeroQualityStillRankable(t *testing.T) {
	p := DefaultParams()
	now := time.Now()
	if w := p.Weight(Candidate{QualityScore: 0, ListedAt: now}, now); w <= 0 {
		t.Fatalf("權重不該是 0：%v", w)
	}
}

func TestQualityScore(t *testing.T) {
	tests := []struct {
		name           string
		worked, failed int64
		wantMin        int32
		wantMax        int32
	}{
		{"沒有回報時等於預設值", 0, 0, DefaultQualityScore, DefaultQualityScore},
		{"單筆成功不該直接滿分", 1, 0, 61, 75},
		{"單筆失敗不該直接歸零", 0, 1, 40, 55},
		{"大量成功趨近滿分", 100, 0, 93, 100},
		{"大量失敗趨近零分", 0, 100, 0, 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := QualityScore(tt.worked, tt.failed)
			if got < tt.wantMin || got > tt.wantMax {
				t.Fatalf("QualityScore(%d, %d) = %d，預期落在 %d~%d",
					tt.worked, tt.failed, got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestShouldAutoDisable(t *testing.T) {
	tests := []struct {
		name          string
		failed, total int64
		want          bool
	}{
		{"沒有回報不下架", 0, 0, false},
		{"失效比例高但筆數不足不下架", 2, 2, false},
		{"達到筆數與比例門檻才下架", 3, 5, true},
		{"筆數夠但比例不足不下架", 3, 10, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShouldAutoDisable(tt.failed, tt.total, 3, 0.6); got != tt.want {
				t.Fatalf("ShouldAutoDisable(%d, %d) = %v，預期 %v", tt.failed, tt.total, got, tt.want)
			}
		})
	}
}
