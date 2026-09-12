package store

import (
	"testing"

	"github.com/google/uuid"

	"refcode-api/internal/store/dbgen"
)

// 後台停權整條線都在 SQL 裡：停權本身、把上架中的碼下架、解除停權時只還「因為
// 停權才被下架」的那批、以及停權的人不能再上架（把關寫在 CreateCode 的
// INSERT ... SELECT FROM users 上）。四件事都沒辦法用假物件驗證，而錯的方向都很貴：
// 漏擋是停權沒生效，過擋是把不該復活的碼放回目錄。
//
// helper（newBlockFixture / user / merchant / code / visible / exec）在
// user_blocks_db_test.go，同一個 package 共用。
//
// 跑法：make test-db

// status 回傳這個使用者現在的帳號狀態。
func (f *blockFixture) status(userID uuid.UUID) string {
	f.t.Helper()
	var s string
	if err := f.st.Pool.QueryRow(f.ctx,
		`SELECT status FROM referral_code_bonus.users WHERE id = $1`, userID).Scan(&s); err != nil {
		f.t.Fatalf("讀帳號狀態失敗: %v", err)
	}
	return s
}

// codeStatus 回傳單一組碼現在的狀態。
func (f *blockFixture) codeStatus(codeID uuid.UUID) string {
	f.t.Helper()
	var s string
	if err := f.st.Pool.QueryRow(f.ctx,
		`SELECT status FROM referral_code_bonus.referral_codes WHERE id = $1`, codeID).Scan(&s); err != nil {
		f.t.Fatalf("讀碼的狀態失敗: %v", err)
	}
	return s
}

// lastReview 回傳這組碼最後一筆軌跡的 action，沒有軌跡回空字串。
// 解除停權靠的就是這個值，所以要驗得到。
func (f *blockFixture) lastReview(codeID uuid.UUID) string {
	f.t.Helper()
	var a string
	err := f.st.Pool.QueryRow(f.ctx,
		`SELECT action FROM referral_code_bonus.code_reviews
		 WHERE code_id = $1 ORDER BY created_at DESC LIMIT 1`, codeID).Scan(&a)
	if err != nil {
		return ""
	}
	return a
}

// review 補一列軌跡。停權流程裡這是 handler 做的事，測試要自己補上，
// 否則 RestoreCodesSuspendedWithUser 判斷不出哪些該還。
func (f *blockFixture) review(codeID uuid.UUID, action string) {
	f.t.Helper()
	f.exec(`INSERT INTO referral_code_bonus.code_reviews (code_id, action, reason)
	        VALUES ($1, $2, '')`, codeID, action)
}

// refreshToken 塞一張未撤銷的 refresh token，用來驗停權會把它撤掉。
func (f *blockFixture) refreshToken(userID uuid.UUID) uuid.UUID {
	f.t.Helper()
	id := uuid.New()
	f.exec(`INSERT INTO referral_code_bonus.refresh_tokens
	          (id, user_id, token_hash, family_id, expires_at)
	        VALUES ($1, $2, $3, $4, now() + interval '30 days')`,
		id, userID, "hash-"+id.String(), uuid.New())
	return id
}

func (f *blockFixture) tokenRevoked(tokenID uuid.UUID) bool {
	f.t.Helper()
	var revoked bool
	err := f.st.Pool.QueryRow(f.ctx,
		`SELECT revoked_at IS NOT NULL FROM referral_code_bonus.refresh_tokens WHERE id = $1`,
		tokenID).Scan(&revoked)
	if err != nil {
		f.t.Fatalf("讀 token 狀態失敗: %v", err)
	}
	return revoked
}

// createCode 走真正的 CreateCode（不是直接 INSERT），因為「停權的人不能上架」
// 這個把關就寫在那支查詢的 WHERE 上。
func (f *blockFixture) createCode(userID, merchantID uuid.UUID, code string) error {
	f.t.Helper()
	_, err := f.st.CreateCode(f.ctx, dbgen.CreateCodeParams{
		UserID:     userID,
		MerchantID: merchantID,
		Code:       code,
		Note:       "",
		CodeType:   "referral",
	})
	return err
}

// TestSuspendUserFullFlow 走一遍完整情境：上架者放三個碼（推薦碼＋折扣碼），
// 後台停權 → 碼從目錄消失、不能再上架 → 解除停權 → 碼回來、又能上架。
func TestSuspendUserFullFlow(t *testing.T) {
	f := newBlockFixture(t)

	uploader := f.user("uploader@example.com")
	other := f.user("other@example.com")

	// 唯一索引是 (user_id, merchant_id, code_type)，三個碼要跨兩家。
	mA := f.merchant("suspend-a")
	mB := f.merchant("suspend-b")
	c1 := f.code(uploader, mA, "referral")
	c2 := f.code(uploader, mA, "discount")
	c3 := f.code(uploader, mB, "discount")

	// 第三方的碼。停權只該動被停權者的碼 —— 沒有這一筆，UPDATE 漏掉
	// user_id 條件也會讓測試通過。
	cOther := f.code(other, mA, "referral")

	token := f.refreshToken(uploader)

	if got := f.visible(mA, nil); got != 3 {
		t.Fatalf("停權前 A 家應該有 3 個碼（上架者 2 + 第三方 1），實際 %d", got)
	}

	// ── 停權 ─────────────────────────────────────────────────────────
	n, err := f.st.SuspendUser(f.ctx, dbgen.SuspendUserParams{ID: uploader})
	if err != nil {
		t.Fatalf("停權失敗: %v", err)
	}
	if n != 1 {
		t.Fatalf("停權應該動到 1 列，實際 %d", n)
	}
	if got := f.status(uploader); got != "suspended" {
		t.Errorf("帳號狀態應該是 suspended，實際 %q", got)
	}

	ids, err := f.st.DisableCodesForSuspendedUser(f.ctx, uploader)
	if err != nil {
		t.Fatalf("下架碼失敗: %v", err)
	}
	if len(ids) != 3 {
		t.Fatalf("應該下架 3 個碼，實際 %d", len(ids))
	}
	for _, id := range ids {
		f.review(id, "suspend")
	}

	if err := f.st.RevokeAllUserTokens(f.ctx, uploader); err != nil {
		t.Fatalf("撤銷 token 失敗: %v", err)
	}

	// 碼從目錄上消失，第三方的不受影響。
	if got := f.visible(mA, nil); got != 1 {
		t.Errorf("停權後 A 家應該只剩第三方那 1 個，實際 %d", got)
	}
	if got := f.visible(mB, nil); got != 0 {
		t.Errorf("停權後 B 家應該一個都沒有，實際 %d", got)
	}
	if got := f.codeStatus(cOther); got != "active" {
		t.Errorf("第三方的碼不該被動到，實際狀態 %q", got)
	}
	for _, id := range []uuid.UUID{c1, c2, c3} {
		if got := f.codeStatus(id); got != "disabled" {
			t.Errorf("碼 %s 應該是 disabled，實際 %q", id, got)
		}
	}
	if !f.tokenRevoked(token) {
		t.Error("停權應該撤掉 refresh token，換不到新的 access token")
	}

	// 停權的人不能再上架 —— 把關在 CreateCode 的 SQL 上，token 沒過期也擋得住。
	if err := f.createCode(uploader, f.merchant("suspend-c"), "NEWCODE"); err == nil {
		t.Error("停權的人上架應該失敗，實際成功了")
	} else if !IsNotFound(err) {
		t.Errorf("停權的人上架應該撈不到列（ErrNoRows），實際 %v", err)
	}

	// ── 解除停權 ─────────────────────────────────────────────────────
	n, err = f.st.ReinstateUser(f.ctx, uploader)
	if err != nil {
		t.Fatalf("解除停權失敗: %v", err)
	}
	if n != 1 {
		t.Fatalf("解除停權應該動到 1 列，實際 %d", n)
	}
	if got := f.status(uploader); got != "active" {
		t.Errorf("帳號狀態應該回到 active，實際 %q", got)
	}

	restored, err := f.st.RestoreCodesSuspendedWithUser(f.ctx, uploader)
	if err != nil {
		t.Fatalf("還原碼失敗: %v", err)
	}
	if len(restored) != 3 {
		t.Fatalf("應該還原 3 個碼，實際 %d", len(restored))
	}
	if got := f.visible(mA, nil); got != 3 {
		t.Errorf("解除停權後 A 家應該回到 3 個，實際 %d", got)
	}

	// 解除停權後又能上架了。
	if err := f.createCode(uploader, f.merchant("suspend-d"), "NEWCODE2"); err != nil {
		t.Errorf("解除停權後上架應該成功，實際 %v", err)
	}
}

// TestSuspendLeavesPendingCodesAlone 驗停權只碰 active 的碼。
//
// pending 的碼在審核佇列裡、目錄上本來就看不到，下架它等於把它從佇列移走，
// 而解除停權時又得決定還他 pending 還是 active —— 後者會讓沒審過的碼直接上架。
func TestSuspendLeavesPendingCodesAlone(t *testing.T) {
	f := newBlockFixture(t)

	uploader := f.user("uploader@example.com")
	m := f.merchant("pending-merchant")

	active := f.code(uploader, m, "referral")
	// pending 的碼直接塞，f.code 建出來的一律是 active。
	pending := uuid.New()
	f.exec(`INSERT INTO referral_code_bonus.referral_codes
	          (id, user_id, merchant_id, code, status, code_type)
	        VALUES ($1, $2, $3, $4, 'pending', 'discount')`,
		pending, uploader, m, "PENDING-1")

	if _, err := f.st.SuspendUser(f.ctx, dbgen.SuspendUserParams{ID: uploader}); err != nil {
		t.Fatalf("停權失敗: %v", err)
	}
	ids, err := f.st.DisableCodesForSuspendedUser(f.ctx, uploader)
	if err != nil {
		t.Fatalf("下架碼失敗: %v", err)
	}

	if len(ids) != 1 || ids[0] != active {
		t.Errorf("只該下架那個 active 的碼，實際下架 %v", ids)
	}
	if got := f.codeStatus(pending); got != "pending" {
		t.Errorf("pending 的碼應該留在佇列裡，實際 %q", got)
	}
}

// TestReinstateOnlyRestoresSuspendedCodes 驗解除停權不會把「本來就被個別下架」
// 的碼一起復活 —— 依據是最後一筆軌跡的 action（見 00020_suspend_action.sql）。
func TestReinstateOnlyRestoresSuspendedCodes(t *testing.T) {
	f := newBlockFixture(t)

	uploader := f.user("uploader@example.com")
	mA := f.merchant("restore-a")
	mB := f.merchant("restore-b")

	// 停權前就被 admin 個別下架的碼，軌跡是 disable。
	individually := f.code(uploader, mA, "referral")
	f.exec(`UPDATE referral_code_bonus.referral_codes SET status = 'disabled' WHERE id = $1`,
		individually)
	f.review(individually, "disable")

	// 被檢舉自動下架的碼，軌跡是 auto_disable。
	autoDisabled := f.code(uploader, mA, "discount")
	f.exec(`UPDATE referral_code_bonus.referral_codes SET status = 'disabled' WHERE id = $1`,
		autoDisabled)
	f.review(autoDisabled, "auto_disable")

	// 這個是停權時才被下架的，軌跡是 suspend。
	bySuspension := f.code(uploader, mB, "referral")

	if _, err := f.st.SuspendUser(f.ctx, dbgen.SuspendUserParams{ID: uploader}); err != nil {
		t.Fatalf("停權失敗: %v", err)
	}
	ids, err := f.st.DisableCodesForSuspendedUser(f.ctx, uploader)
	if err != nil {
		t.Fatalf("下架碼失敗: %v", err)
	}
	if len(ids) != 1 || ids[0] != bySuspension {
		t.Fatalf("停權只該下架那個 active 的碼，實際 %v", ids)
	}
	for _, id := range ids {
		f.review(id, "suspend")
	}

	if _, err := f.st.ReinstateUser(f.ctx, uploader); err != nil {
		t.Fatalf("解除停權失敗: %v", err)
	}
	restored, err := f.st.RestoreCodesSuspendedWithUser(f.ctx, uploader)
	if err != nil {
		t.Fatalf("還原碼失敗: %v", err)
	}

	if len(restored) != 1 || restored[0] != bySuspension {
		t.Errorf("只該還原停權下架的那一個，實際 %v", restored)
	}
	if got := f.codeStatus(individually); got != "disabled" {
		t.Errorf("個別下架的碼不該復活，實際 %q", got)
	}
	if got := f.codeStatus(autoDisabled); got != "disabled" {
		t.Errorf("檢舉自動下架的碼不該復活，實際 %q", got)
	}
}

// TestSuspendReinstateIdempotent 驗兩支 UPDATE 的狀態條件：重複停權、
// 解除沒被停權的人，都要回 0 列而不是默默成功。handler 靠這個回傳值分辨
// 「已經是這個狀態了」，回錯會讓重按一次看起來像系統壞了。
func TestSuspendReinstateIdempotent(t *testing.T) {
	f := newBlockFixture(t)

	u := f.user("u@example.com")

	if n, err := f.st.SuspendUser(f.ctx, dbgen.SuspendUserParams{ID: u}); err != nil || n != 1 {
		t.Fatalf("第一次停權應該動到 1 列，實際 n=%d err=%v", n, err)
	}
	if n, err := f.st.SuspendUser(f.ctx, dbgen.SuspendUserParams{ID: u}); err != nil || n != 0 {
		t.Errorf("重複停權應該 0 列，實際 n=%d err=%v", n, err)
	}

	if n, err := f.st.ReinstateUser(f.ctx, u); err != nil || n != 1 {
		t.Fatalf("第一次解除應該動到 1 列，實際 n=%d err=%v", n, err)
	}
	if n, err := f.st.ReinstateUser(f.ctx, u); err != nil || n != 0 {
		t.Errorf("解除沒被停權的人應該 0 列，實際 n=%d err=%v", n, err)
	}

	// 不存在的人也是 0 列，不是錯誤。
	if n, err := f.st.SuspendUser(f.ctx, dbgen.SuspendUserParams{ID: uuid.New()}); err != nil || n != 0 {
		t.Errorf("停權不存在的人應該 0 列，實際 n=%d err=%v", n, err)
	}
}

// TestSuspendedUserCannotCreateCode 單獨驗把關在 SQL 上，而不是靠 handler。
// 這是停權之後那段 access token 還沒過期的空窗期唯一的防線。
func TestSuspendedUserCannotCreateCode(t *testing.T) {
	f := newBlockFixture(t)

	u := f.user("u@example.com")
	m := f.merchant("guard-merchant")

	if err := f.createCode(u, m, "OK-1"); err != nil {
		t.Fatalf("正常使用者上架應該成功，實際 %v", err)
	}

	if _, err := f.st.SuspendUser(f.ctx, dbgen.SuspendUserParams{ID: u}); err != nil {
		t.Fatalf("停權失敗: %v", err)
	}

	err := f.createCode(u, f.merchant("guard-merchant-2"), "BLOCKED-1")
	if err == nil {
		t.Fatal("停權的人上架應該失敗，實際成功了")
	}
	if !IsNotFound(err) {
		t.Errorf("應該是撈不到列（ErrNoRows），實際 %v", err)
	}
}
