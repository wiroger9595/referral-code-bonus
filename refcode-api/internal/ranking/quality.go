package ranking

// DefaultQualityScore 是還沒有任何回報時的分數。
// 給中間偏上：新碼要有曝光機會，但不該壓過已經被驗證有效的碼。
const DefaultQualityScore = 60

// 貝氏平滑的先驗：相當於一開始就有 5 筆回報、其中 3 筆成功。
// 直接用成功率會讓「1 筆成功」變成滿分、「1 筆失敗」變成 0 分，
// 少量樣本的雜訊會整個蓋過排序。
const (
	priorWeight  = 5.0
	priorSuccess = 3.0
)

// QualityScore 由回報數據算出 0~100 的分數。
func QualityScore(worked, failed int64) int32 {
	total := float64(worked + failed)
	score := 100 * (float64(worked) + priorSuccess) / (total + priorWeight)

	switch {
	case score < 0:
		return 0
	case score > 100:
		return 100
	}
	return int32(score + 0.5)
}

// ShouldAutoDisable 判斷是否自動下架。
//
// 只看最近 N 筆回報（SQL 那邊限 10 筆），因為服務商可能重啟活動，
// 舊的失效紀錄不該永遠壓著一個碼。同時要求絕對筆數門檻，
// 否則兩個人惡意回報就能打掉競爭對手的碼。
func ShouldAutoDisable(failed, total int64, minReports int, failRatio float64) bool {
	if total == 0 || failed < int64(minReports) {
		return false
	}
	return float64(failed)/float64(total) >= failRatio
}
