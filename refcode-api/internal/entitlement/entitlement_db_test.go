package entitlement

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"refcode-api/internal/store"
)

// 這一組是唯一要連資料庫的測試。降級的核心判斷（留最舊的幾個）寫在 SQL 的
// row_number() 裡，恢復那邊的「撞唯一索引只跳過單筆」也只有真的撞了才會發生 ——
// 兩者都沒辦法用假物件驗證，錯了又是使用者的碼默默消失或默默重複上架。
//
// 跑法：make test-db（會自己建 refcode_test 並套用 migration）。
// 沒有 TEST_DATABASE_URL 就整組跳過，所以 make test 維持離線可跑。
//
// TEST_DATABASE_URL 一定要指向本機的 throwaway database —— 這裡會清表，
// 指到 Supabase 那台會直接清掉正式資料。

func testStore(t *testing.T) *store.Store {
	t.Helper()

	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("沒有 TEST_DATABASE_URL，跳過連資料庫的測試（make test-db 會設）")
	}

	st, err := store.New(context.Background(), url)
	if err != nil {
		t.Fatalf("連測試資料庫失敗: %v", err)
	}
	t.Cleanup(st.Close)

	// 每個測試自己開場清空，而不是結束時清 —— 上一輪 panic 留下的殘骸
	// 不該讓下一輪跟著失敗。
	_, err = st.Pool.Exec(context.Background(), `
		TRUNCATE referral_code_bonus.code_reviews,
		         referral_code_bonus.referral_codes,
		         referral_code_bonus.subscriptions,
		         referral_code_bonus.merchants,
		         referral_code_bonus.merchant_categories,
		         referral_code_bonus.users
		CASCADE`)
	if err != nil {
		t.Fatalf("清空測試資料失敗: %v", err)
	}
	return st
}

// fixture 把重複的 INSERT 收在一起，讓每個 case 只剩下它真正在描述的情境。
type fixture struct {
	t   *testing.T
	st  *store.Store
	ctx context.Context
}

func newFixture(t *testing.T) *fixture {
	return &fixture{t: t, st: testStore(t), ctx: context.Background()}
}

func (f *fixture) exec(sql string, args ...any) {
	f.t.Helper()
	if _, err := f.st.Pool.Exec(f.ctx, sql, args...); err != nil {
		f.t.Fatalf("塞測試資料失敗: %v\nSQL: %s", err, sql)
	}
}

func (f *fixture) user(email string) uuid.UUID {
	f.t.Helper()
	id := uuid.New()
	f.exec(`INSERT INTO referral_code_bonus.users (id, email) VALUES ($1, $2)`, id, email)
	return id
}

// merchant 每次都新開一家：唯一索引是 (user_id, merchant_id, code_type)，
// 想讓同一個人有多個同時上架的碼就得換服務商。
func (f *fixture) merchant(slug string) uuid.UUID {
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

// code 直接指定 created_at 與 id，因為「留最舊」跟「同時間用 id 決勝」
// 兩條規則都靠這兩個欄位，不能交給 default 隨機決定。
func (f *fixture) code(id, userID, merchantID uuid.UUID, status string, createdAt time.Time, activated bool) {
	f.t.Helper()
	var activatedAt *time.Time
	if activated {
		activatedAt = &createdAt
	}
	f.exec(`INSERT INTO referral_code_bonus.referral_codes
	          (id, user_id, merchant_id, code, status, created_at, activated_at, expires_at, code_type)
	        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'referral')`,
		id, userID, merchantID, "CODE-"+id.String()[:8], status, createdAt, activatedAt,
		createdAt.Add(365*24*time.Hour))
}

func (f *fixture) review(codeID uuid.UUID, action string, at time.Time) {
	f.t.Helper()
	f.exec(`INSERT INTO referral_code_bonus.code_reviews (code_id, action, reason, created_at)
	        VALUES ($1, $2, '', $3)`, codeID, action, at)
}

func (f *fixture) subscription(userID uuid.UUID, isActive bool, expiresAt *time.Time) {
	f.t.Helper()
	f.exec(`INSERT INTO referral_code_bonus.subscriptions
	          (user_id, entitlement, product_id, store, is_active, will_renew, rc_app_user_id, expires_at)
	        VALUES ($1, 'refcode_pro', 'pro_monthly', 'app_store', $2, true, $3, $4)`,
		userID, isActive, userID.String(), expiresAt)
}

func (f *fixture) status(codeID uuid.UUID) string {
	f.t.Helper()
	var s string
	err := f.st.Pool.QueryRow(f.ctx,
		`SELECT status FROM referral_code_bonus.referral_codes WHERE id = $1`, codeID).Scan(&s)
	if err != nil {
		f.t.Fatalf("讀狀態失敗: %v", err)
	}
	return s
}

func (f *fixture) lastAction(codeID uuid.UUID) string {
	f.t.Helper()
	var a string
	err := f.st.Pool.QueryRow(f.ctx,
		`SELECT action FROM referral_code_bonus.code_reviews
		 WHERE code_id = $1 ORDER BY created_at DESC LIMIT 1`, codeID).Scan(&a)
	if err != nil {
		f.t.Fatalf("讀軌跡失敗: %v", err)
	}
	return a
}

func (f *fixture) reviewCount(codeID uuid.UUID) int {
	f.t.Helper()
	var n int
	if err := f.st.Pool.QueryRow(f.ctx,
		`SELECT count(*) FROM referral_code_bonus.code_reviews WHERE code_id = $1`,
		codeID).Scan(&n); err != nil {
		f.t.Fatalf("數軌跡失敗: %v", err)
	}
	return n
}

// 這是「留最舊」這個決定唯一的把關處。撤錯邊（留最新）不會有任何錯誤訊息，
// 只會讓使用者最早經營、累積過曝光的碼默默消失。
func TestDowngradeKeepsOldest(t *testing.T) {
	f := newFixture(t)
	userID := f.user("keep-oldest@example.com")
	base := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)

	// 五個碼，由舊到新。第三個刻意留 pending：額度算的是 pending + active 相加，
	// 只算 active 的話永遠收斂不到 3 個。
	ids := make([]uuid.UUID, 5)
	for i := range ids {
		ids[i] = uuid.New()
		status := "active"
		if i == 2 {
			status = "pending"
		}
		f.code(ids[i], userID, f.merchant("m-keep-oldest-"+string(rune('a'+i))),
			status, base.Add(time.Duration(i)*time.Hour), status == "active")
	}

	n, err := New(f.st, 3).Downgrade(f.ctx, userID)
	if err != nil {
		t.Fatalf("Downgrade 失敗: %v", err)
	}
	if n != 2 {
		t.Errorf("撤掉的數量 = %d, 預期 2", n)
	}

	// 前三個（最舊）留著，而且狀態原封不動 —— pending 不該被順手變成 active。
	want := []string{"active", "active", "pending", "disabled", "disabled"}
	for i, id := range ids {
		if got := f.status(id); got != want[i] {
			t.Errorf("第 %d 舊的碼狀態 = %q, 預期 %q", i+1, got, want[i])
		}
	}

	// 被撤的要留 downgrade 軌跡，續訂恢復時就是靠這個認人。
	for _, id := range ids[3:] {
		if got := f.lastAction(id); got != "downgrade" {
			t.Errorf("被撤的碼最後一筆軌跡 = %q, 預期 downgrade", got)
		}
	}
	// 留下來的不該有任何軌跡，不然會被恢復流程誤認。
	for i, id := range ids[:3] {
		if got := f.reviewCount(id); got != 0 {
			t.Errorf("留下來的第 %d 個碼被寫了 %d 筆軌跡，應該是 0", i+1, got)
		}
	}
}

// created_at 撞在一起（同一批匯入、seed 塞的）時要有穩定的勝負，
// 不然同一份資料每次跑撤掉的碼不一樣，客訴根本查不出來。
func TestDowngradeTieBreaksByID(t *testing.T) {
	f := newFixture(t)
	userID := f.user("tie@example.com")
	sameTime := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)

	// uuid 隨機，先排好再指定誰是小的那個。
	a, b := uuid.New(), uuid.New()
	if b.String() < a.String() {
		a, b = b, a
	}
	f.code(a, userID, f.merchant("m-tie-a"), "active", sameTime, true)
	f.code(b, userID, f.merchant("m-tie-b"), "active", sameTime, true)

	if _, err := New(f.st, 1).Downgrade(f.ctx, userID); err != nil {
		t.Fatalf("Downgrade 失敗: %v", err)
	}

	if got := f.status(a); got != "active" {
		t.Errorf("id 較小的碼狀態 = %q, 預期 active（決勝要穩定）", got)
	}
	if got := f.status(b); got != "disabled" {
		t.Errorf("id 較大的碼狀態 = %q, 預期 disabled", got)
	}
}

// 恢復要回哪個狀態、以及哪些碼根本不該被恢復。
func TestRestore(t *testing.T) {
	f := newFixture(t)
	userID := f.user("restore@example.com")
	base := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)

	// 曾經上架過 → 回 active。
	wasLive := uuid.New()
	f.code(wasLive, userID, f.merchant("m-restore-live"), "disabled", base, true)
	f.review(wasLive, "downgrade", base.Add(time.Hour))

	// 還在審核就被撤 → 回 pending，不能跳過審核。
	neverLive := uuid.New()
	f.code(neverLive, userID, f.merchant("m-restore-pending"), "disabled", base, false)
	f.review(neverLive, "downgrade", base.Add(time.Hour))

	// 降級撤掉之後使用者又自己按了下架 → 最後一筆不是 downgrade，不該被續訂帶回架上。
	thenSelfDisabled := uuid.New()
	f.code(thenSelfDisabled, userID, f.merchant("m-restore-self"), "disabled", base, true)
	f.review(thenSelfDisabled, "downgrade", base.Add(time.Hour))
	f.review(thenSelfDisabled, "disable", base.Add(2*time.Hour))

	// 被品質分數自動下架的，同理不該恢復。
	autoDisabled := uuid.New()
	f.code(autoDisabled, userID, f.merchant("m-restore-auto"), "disabled", base, true)
	f.review(autoDisabled, "auto_disable", base.Add(time.Hour))

	n, err := New(f.st, 3).Restore(f.ctx, userID)
	if err != nil {
		t.Fatalf("Restore 失敗: %v", err)
	}
	if n != 2 {
		t.Errorf("恢復的數量 = %d, 預期 2", n)
	}

	cases := []struct {
		name string
		id   uuid.UUID
		want string
	}{
		{"曾經上架過的回架上", wasLive, "active"},
		{"沒過審的回審核佇列", neverLive, "pending"},
		{"自己下架的不動", thenSelfDisabled, "disabled"},
		{"品質自動下架的不動", autoDisabled, "disabled"},
	}
	for _, c := range cases {
		if got := f.status(c.id); got != c.want {
			t.Errorf("%s: 狀態 = %q, 預期 %q", c.name, got, c.want)
		}
	}
}

// 降級期間使用者在同一家、同類型重新上架了一個新碼，舊的恢復回去會撞
// codes_user_merchant_type_live_idx。那一筆要單獨跳過，不能讓整批恢復回滾 ——
// 一筆撞到就全滾的話，使用者付了錢卻一個碼都沒回來。
func TestRestoreSkipsConflictOnly(t *testing.T) {
	f := newFixture(t)
	userID := f.user("conflict@example.com")
	base := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	clashMerchant := f.merchant("m-conflict")

	// 被降級撤掉的舊碼。
	old := uuid.New()
	f.code(old, userID, clashMerchant, "disabled", base, true)
	f.review(old, "downgrade", base.Add(time.Hour))

	// 降級之後在同一家重新上架的新碼，這才是他現在在用的。
	replacement := uuid.New()
	f.code(replacement, userID, clashMerchant, "active", base.Add(2*time.Hour), true)

	// 另一家的降級碼，這個要正常恢復 —— 用來證明不是整批回滾。
	other := uuid.New()
	f.code(other, userID, f.merchant("m-conflict-other"), "disabled", base, true)
	f.review(other, "downgrade", base.Add(time.Hour))

	n, err := New(f.st, 3).Restore(f.ctx, userID)
	if err != nil {
		t.Fatalf("撞唯一索引不該讓整個 Restore 失敗: %v", err)
	}
	if n != 1 {
		t.Errorf("恢復的數量 = %d, 預期 1（撞到的那筆跳過）", n)
	}

	if got := f.status(old); got != "disabled" {
		t.Errorf("撞到唯一索引的舊碼狀態 = %q, 預期維持 disabled", got)
	}
	if got := f.status(replacement); got != "active" {
		t.Errorf("降級期間重新上架的碼狀態 = %q, 預期 active（不該被擠掉）", got)
	}
	if got := f.status(other); got != "active" {
		t.Errorf("另一家的降級碼狀態 = %q, 預期 active", got)
	}
}

// Sweep 是 webhook 漏掉時的兜底，兩支篩選 SQL 挑錯人不會有錯誤訊息 ——
// 挑多了會撤掉付費使用者的碼，挑少了等於沒兜底。
func TestSweep(t *testing.T) {
	f := newFixture(t)
	base := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	// 訂閱的到期日是跟 now() 比的，不能沿用上面那個固定的 base。
	future := time.Now().Add(365 * 24 * time.Hour)
	past := time.Now().Add(-24 * time.Hour)

	// 沒有訂閱、超額 → 要被降級。
	overLimit := f.user("over@example.com")
	overIDs := make([]uuid.UUID, 5)
	for i := range overIDs {
		overIDs[i] = uuid.New()
		f.code(overIDs[i], overLimit, f.merchant("m-sweep-over-"+string(rune('a'+i))),
			"active", base.Add(time.Duration(i)*time.Hour), true)
	}

	// 訂閱過期了但 is_active 還是 true（webhook 沒送到）→ 一樣要被降級。
	// 這是篩選條件最容易寫錯的地方：只看 is_active 會漏掉這種人。
	lapsed := f.user("lapsed@example.com")
	f.subscription(lapsed, true, &past)
	lapsedIDs := make([]uuid.UUID, 4)
	for i := range lapsedIDs {
		lapsedIDs[i] = uuid.New()
		f.code(lapsedIDs[i], lapsed, f.merchant("m-sweep-lapsed-"+string(rune('a'+i))),
			"active", base.Add(time.Duration(i)*time.Hour), true)
	}

	// Pro 生效、身上還有降級碼 → 要被恢復。
	pro := f.user("pro@example.com")
	f.subscription(pro, true, &future)
	proCode := uuid.New()
	f.code(proCode, pro, f.merchant("m-sweep-pro"), "disabled", base, true)
	f.review(proCode, "downgrade", base.Add(time.Hour))

	// 沒有訂閱但沒超額 → 一個都不能動。
	underLimit := f.user("under@example.com")
	underIDs := make([]uuid.UUID, 2)
	for i := range underIDs {
		underIDs[i] = uuid.New()
		f.code(underIDs[i], underLimit, f.merchant("m-sweep-under-"+string(rune('a'+i))),
			"active", base.Add(time.Duration(i)*time.Hour), true)
	}

	downgraded, restored, err := New(f.st, 3).Sweep(f.ctx)
	if err != nil {
		t.Fatalf("Sweep 失敗: %v", err)
	}
	if downgraded != 2 {
		t.Errorf("降級的使用者數 = %d, 預期 2（超額 + 訂閱已過期）", downgraded)
	}
	if restored != 1 {
		t.Errorf("恢復的使用者數 = %d, 預期 1", restored)
	}

	for i, id := range overIDs {
		want := "active"
		if i >= 3 {
			want = "disabled"
		}
		if got := f.status(id); got != want {
			t.Errorf("超額使用者第 %d 舊的碼 = %q, 預期 %q", i+1, got, want)
		}
	}
	for i, id := range lapsedIDs {
		want := "active"
		if i >= 3 {
			want = "disabled"
		}
		if got := f.status(id); got != want {
			t.Errorf("訂閱過期使用者第 %d 舊的碼 = %q, 預期 %q", i+1, got, want)
		}
	}
	if got := f.status(proCode); got != "active" {
		t.Errorf("Pro 使用者的降級碼 = %q, 預期 active", got)
	}
	for i, id := range underIDs {
		if got := f.status(id); got != "active" {
			t.Errorf("沒超額使用者第 %d 個碼 = %q, 預期 active（不該被動）", i+1, got)
		}
	}
}
