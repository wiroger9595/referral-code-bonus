package ranking

import (
	"math/rand/v2"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestWeightFavorsLowExposure(t *testing.T) {
	p := DefaultParams()
	now := time.Now()
	created := now.Add(-30 * 24 * time.Hour) // 抽掉新鮮度加成的影響

	fresh := Candidate{QualityScore: 60, Impressions: 0, CreatedAt: created}
	saturated := Candidate{QualityScore: 60, Impressions: 1000, CreatedAt: created}

	if p.Weight(fresh, now) <= p.Weight(saturated, now) {
		t.Fatalf("曝光少的碼權重應該較高：fresh=%v saturated=%v",
			p.Weight(fresh, now), p.Weight(saturated, now))
	}
}

// 曝光懲罰是這套排序的重點：品質相同時，已經吃掉大量曝光的碼
// 必須把位置讓給還沒被看過的碼，否則新上架的人永遠排不進去。
func TestRankGivesNewcomersRealChance(t *testing.T) {
	now := time.Now()
	created := now.Add(-30 * 24 * time.Hour)

	newcomer := Candidate{ID: uuid.New(), QualityScore: 60, Impressions: 0, CreatedAt: created}
	candidates := []Candidate{newcomer}
	for i := 0; i < 9; i++ {
		candidates = append(candidates, Candidate{
			ID: uuid.New(), QualityScore: 60, Impressions: 2000, CreatedAt: created,
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
		candidates[i] = Candidate{ID: uuid.New(), QualityScore: int32(i + 1), CreatedAt: now}
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

// 品質分數為 0 的碼仍要留最低權重，否則一旦沉底就再也拿不到翻身的曝光。
func TestZeroQualityStillRankable(t *testing.T) {
	p := DefaultParams()
	now := time.Now()
	if w := p.Weight(Candidate{QualityScore: 0, CreatedAt: now}, now); w <= 0 {
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
