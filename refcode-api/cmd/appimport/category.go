package main

// App Store 的分類跟我們的目錄分類不是一對一。這張表只收「這類服務常有推薦計畫」
// 的 genre，其餘（遊戲、社群、工具…）一律跳過 —— 排行榜前段大半是手遊，
// 全部倒進來只會讓目錄變成沒人能填推薦碼的空殼清單。
//
// 對應的是 Lookup 回傳的 primaryGenreName（固定英文），不是 genres
// （那個會跟著 country 在地化，比對起來會隨國別壞掉）。
var genreToCategory = map[string]category{
	"Finance":          {name: "銀行信用卡", nameEn: "Banking & Credit Cards", nameJa: "銀行・クレジットカード", sort: 0},
	"Food & Drink":     {name: "外送", nameEn: "Food Delivery", nameJa: "フードデリバリー", sort: 2},
	"Entertainment":    {name: "影音串流", nameEn: "Streaming", nameJa: "動画・音楽配信", sort: 3},
	"Music":            {name: "影音串流", nameEn: "Streaming", nameJa: "動画・音楽配信", sort: 3},
	"Photo & Video":    {name: "影音串流", nameEn: "Streaming", nameJa: "動画・音楽配信", sort: 3},
	"Shopping":         {name: "購物", nameEn: "Shopping", nameJa: "ショッピング", sort: 4},
	"Travel":           {name: "旅遊訂房", nameEn: "Travel & Hotels", nameJa: "旅行・ホテル", sort: 5},
	"Navigation":       {name: "交通出行", nameEn: "Rides & Transport", nameJa: "交通・移動", sort: 6},
	"Health & Fitness": {name: "健康運動", nameEn: "Health & Fitness", nameJa: "健康・フィットネス", sort: 7},
	"Education":        {name: "學習教育", nameEn: "Learning", nameJa: "学習・教育", sort: 8},
	"Productivity":     {name: "工具與生產力", nameEn: "Productivity", nameJa: "仕事効率化", sort: 9},
	"Business":         {name: "工具與生產力", nameEn: "Productivity", nameJa: "仕事効率化", sort: 9},
}

type category struct {
	name   string
	nameEn string
	nameJa string
	sort   int32
}
