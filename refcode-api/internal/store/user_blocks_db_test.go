package store

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"refcode-api/internal/store/dbgen"
)

// 封鎖的語意是「被我封鎖的人，他的碼不再出現在我眼前」（見 00012_user_blocks.sql），
// 而它整條線都在 SQL 裡：過濾寫在 ListActiveCodesForMerchant 的 NOT EXISTS 上。
// 沒辦法用假物件驗證 —— 條件寫錯的兩種後果都不會有人回報：漏擋是封鎖沒生效，
// 過擋是把沒被封鎖的人的碼一起弄消失。
//
// 跑法：make test-db（會自己建 refcode_test 並套用 migration）。
// 沒有 TEST_DATABASE_URL 就整組跳過，所以 make test 維持離線可跑。

func blockStore(t *testing.T) *Store {
	t.Helper()

	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("沒有 TEST_DATABASE_URL，跳過連資料庫的測試（make test-db 會設）")
	}

	st, err := New(context.Background(), url)
	if err != nil {
		t.Fatalf("連測試資料庫失敗: %v", err)
	}
	t.Cleanup(st.Close)

	// 開場清空，而不是結束時清 —— 上一輪 panic 留下的殘骸不該讓下一輪跟著失敗。
	_, err = st.Pool.Exec(context.Background(), `
		TRUNCATE referral_code_bonus.user_blocks,
		         referral_code_bonus.referral_codes,
		         referral_code_bonus.merchants,
		         referral_code_bonus.merchant_categories,
		         referral_code_bonus.users
		CASCADE`)
	if err != nil {
		t.Fatalf("清空測試資料失敗: %v", err)
	}
	return st
}

type blockFixture struct {
	t   *testing.T
	st  *Store
	ctx context.Context
}

func newBlockFixture(t *testing.T) *blockFixture {
	return &blockFixture{t: t, st: blockStore(t), ctx: context.Background()}
}

func (f *blockFixture) exec(sql string, args ...any) {
	f.t.Helper()
	if _, err := f.st.Pool.Exec(f.ctx, sql, args...); err != nil {
		f.t.Fatalf("塞測試資料失敗: %v\nSQL: %s", err, sql)
	}
}

func (f *blockFixture) user(email string) uuid.UUID {
	f.t.Helper()
	id := uuid.New()
	f.exec(`INSERT INTO referral_code_bonus.users (id, email, display_name) VALUES ($1, $2, $3)`,
		id, email, email)
	return id
}

func (f *blockFixture) merchant(slug string) uuid.UUID {
	f.t.Helper()
	catID := uuid.New()
	f.exec(`INSERT INTO referral_code_bonus.merchant_categories (id, name) VALUES ($1, $2)`,
		catID, "測試分類-"+slug)
	id := uuid.New()
	f.exec(`INSERT INTO referral_code_bonus.merchants (id, slug, name, category_id, signup_url)
	        VALUES ($1, $2, $3, $4, $5)`,
		id, slug, "測試服務商-"+slug, catID, "https://example.com/"+slug)
	return id
}

// code 上架一個 active 的碼。
//
// 兩種 code_type 的欄位一模一樣：00014 加的 discount_kind／value／currency 在
// 00015 又全部收回去了，折扣的優惠內容現在寫在 note 裡。所以這裡只需要換
// code_type，不必為折扣碼多填任何東西。
func (f *blockFixture) code(userID, merchantID uuid.UUID, codeType string) uuid.UUID {
	f.t.Helper()
	id := uuid.New()

	prefix := "REF-"
	note := ""
	if codeType == "discount" {
		prefix = "DISC-"
		note = "全站 8 折"
	}
	f.exec(`INSERT INTO referral_code_bonus.referral_codes
	          (id, user_id, merchant_id, code, note, status, activated_at, code_type)
	        VALUES ($1, $2, $3, $4, $5, 'active', $6, $7)`,
		id, userID, merchantID, prefix+id.String()[:8], note, time.Now(), codeType)
	return id
}

// visible 回傳某個人在某家服務商頁上看得到幾個碼。viewer 傳 nil 代表沒登入。
func (f *blockFixture) visible(merchantID uuid.UUID, viewer *uuid.UUID) int {
	f.t.Helper()
	rows, err := f.st.ListActiveCodesForMerchant(f.ctx, dbgen.ListActiveCodesForMerchantParams{
		MerchantID: merchantID,
		ViewerID:   viewer,
	})
	if err != nil {
		f.t.Fatalf("撈服務商頁的碼失敗: %v", err)
	}
	return len(rows)
}

// TestUserBlockHidesOwnerCodes 走一遍實際情境：一個上架者放了三個碼
// （推薦碼與折扣碼混合），被某個使用者封鎖之後，那三個碼只從封鎖者眼前消失。
func TestUserBlockHidesOwnerCodes(t *testing.T) {
	f := newBlockFixture(t)

	uploader := f.user("uploader@example.com")
	viewer := f.user("viewer@example.com")
	other := f.user("other@example.com")

	// 唯一索引是 (user_id, merchant_id, code_type)，所以同一家只能各放一種；
	// 三個碼要跨兩家。A 家刻意放兩個，用來驗封鎖是把這個人的碼全部拿掉。
	mA := f.merchant("merchant-a")
	mB := f.merchant("merchant-b")
	f.code(uploader, mA, "referral")
	f.code(uploader, mA, "discount")
	f.code(uploader, mB, "discount")

	// 第三方的碼。封鎖只該拿掉被封鎖者的碼，不是把整頁清空 ——
	// 沒有這一筆，NOT EXISTS 寫成無條件成立也會讓測試通過。
	f.code(other, mA, "referral")

	if got := f.visible(mA, &viewer); got != 3 {
		t.Fatalf("封鎖前 A 家應該看到 3 個碼（上架者 2 + 第三方 1），實際 %d", got)
	}
	if got := f.visible(mB, &viewer); got != 1 {
		t.Fatalf("封鎖前 B 家應該看到 1 個碼，實際 %d", got)
	}

	if err := f.st.BlockUser(f.ctx, dbgen.BlockUserParams{
		BlockerID: viewer, BlockedID: uploader,
	}); err != nil {
		t.Fatalf("封鎖失敗: %v", err)
	}

	if got := f.visible(mA, &viewer); got != 1 {
		t.Errorf("封鎖後 A 家應該只剩第三方那 1 個碼，實際 %d", got)
	}
	if got := f.visible(mB, &viewer); got != 0 {
		t.Errorf("封鎖後 B 家應該一個都看不到，實際 %d", got)
	}

	// 封鎖是單向的，也不影響對方帳號（見 00012 的註解）：別人與沒登入的訪客
	// 看到的東西完全不變。
	if got := f.visible(mA, &other); got != 3 {
		t.Errorf("第三方不該受別人的封鎖影響，A 家仍應看到 3 個，實際 %d", got)
	}
	if got := f.visible(mA, nil); got != 3 {
		t.Errorf("未登入沒有封鎖名單可言，A 家仍應看到 3 個，實際 %d", got)
	}
	if got := f.visible(mA, &uploader); got != 3 {
		t.Errorf("被封鎖的人自己看目錄不該有變化，A 家仍應看到 3 個，實際 %d", got)
	}

	// 解除封鎖要救得回來 —— 沒有這一段，封鎖等於不可逆。
	if err := f.st.UnblockUser(f.ctx, dbgen.UnblockUserParams{
		BlockerID: viewer, BlockedID: uploader,
	}); err != nil {
		t.Fatalf("解除封鎖失敗: %v", err)
	}
	if got := f.visible(mA, &viewer); got != 3 {
		t.Errorf("解除封鎖後 A 家應該恢復 3 個，實際 %d", got)
	}
}

// TestUserBlockIdempotentAndNotSelf 驗兩個約束：重複封鎖同一個人不是錯誤，
// 自己封鎖自己要被資料庫擋下來（會讓「我的推薦碼」憑空消失）。
func TestUserBlockIdempotentAndNotSelf(t *testing.T) {
	f := newBlockFixture(t)

	a := f.user("a@example.com")
	b := f.user("b@example.com")

	for i := 0; i < 2; i++ {
		if err := f.st.BlockUser(f.ctx, dbgen.BlockUserParams{BlockerID: a, BlockedID: b}); err != nil {
			t.Fatalf("第 %d 次封鎖失敗（ON CONFLICT DO NOTHING 應該吃掉重複）: %v", i+1, err)
		}
	}
	blocks, err := f.st.ListMyBlocks(f.ctx, a)
	if err != nil {
		t.Fatalf("讀封鎖名單失敗: %v", err)
	}
	if len(blocks) != 1 {
		t.Errorf("重複封鎖只該留一列，實際 %d", len(blocks))
	}

	if err := f.st.BlockUser(f.ctx, dbgen.BlockUserParams{BlockerID: a, BlockedID: a}); err == nil {
		t.Error("自己封鎖自己應該被 user_blocks_not_self 擋下來，實際成功了")
	}
}

// TestBlockDoesNotAffectMerchantCodeCount 記錄一個目前的不一致：服務商卡片上的
// 「N 個可用碼」不看封鎖名單。
//
// ListMerchantsParams 沒有 viewer_id，這個數字在 SQL 層就不可能扣掉被封鎖的人 ——
// 所以封鎖之後目錄上仍顯示 3 個可用碼，點進去卻只剩 1 個。
//
// 這個測試斷言的是「現況」而不是「應該的行為」。哪天決定要修，這裡會紅，
// 那正是提醒：要改的是 ListMerchants 收 viewer_id，不是改這個斷言。
func TestBlockDoesNotAffectMerchantCodeCount(t *testing.T) {
	f := newBlockFixture(t)

	uploader := f.user("uploader@example.com")
	viewer := f.user("viewer@example.com")
	other := f.user("other@example.com")

	m := f.merchant("merchant-count")
	f.code(uploader, m, "referral")
	f.code(uploader, m, "discount")
	f.code(other, m, "referral")

	if err := f.st.BlockUser(f.ctx, dbgen.BlockUserParams{
		BlockerID: viewer, BlockedID: uploader,
	}); err != nil {
		t.Fatalf("封鎖失敗: %v", err)
	}

	if got := f.visible(m, &viewer); got != 1 {
		t.Fatalf("封鎖後服務商頁應該只剩 1 個碼，實際 %d", got)
	}

	rows, err := f.st.ListMerchants(f.ctx, dbgen.ListMerchantsParams{Limit: 10})
	if err != nil {
		t.Fatalf("撈服務商列表失敗: %v", err)
	}
	var count int64 = -1
	for _, r := range rows {
		if r.ID == m {
			count = r.ActiveCodeCount
		}
	}
	if count != 3 {
		t.Errorf("目前的 active_code_count 不看封鎖名單，預期仍是 3（與頁內的 1 個不一致），實際 %d", count)
	}
}
