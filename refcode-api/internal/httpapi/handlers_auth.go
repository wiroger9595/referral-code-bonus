package httpapi

import (
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"

	"refcode-api/internal/auth"
	"refcode-api/internal/geo"
	"refcode-api/internal/mailer"
	"refcode-api/internal/store"
	"refcode-api/internal/store/dbgen"
)

type userResponse struct {
	ID          uuid.UUID `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	AvatarURL   *string   `json:"avatar_url"`
	Verified    bool      `json:"email_verified"`
	Country     *string   `json:"country"`
	CreatedAt   time.Time `json:"created_at"`

	// 訂閱狀態。app 端的畫面以 RevenueCat SDK 為準（那才是即時的），
	// 這兩個欄位是伺服器端這份副本，讓 app 能發現兩邊不一致。
	// 只有 /v1/me 會填，登入當下 webhook 可能還沒到。
	IsPro        bool       `json:"is_pro"`
	ProExpiresAt *time.Time `json:"pro_expires_at"`
}

// dbgen.User 帶著 password_hash，而且 sqlc 產的 struct 有 json tag，
// 直接回給前端就會外洩。所有回傳使用者資料的地方都要經過這裡。
func toUserResponse(u dbgen.User) userResponse {
	return userResponse{
		ID:          u.ID,
		Email:       u.Email,
		DisplayName: u.DisplayName,
		AvatarURL:   u.AvatarUrl,
		Verified:    u.EmailVerifiedAt != nil,
		Country:     u.Country,
		CreatedAt:   u.CreatedAt,
	}
}

type authResponse struct {
	User   userResponse    `json:"user"`
	Tokens *auth.TokenPair `json:"tokens"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email       string `json:"email"`
		Password    string `json:"password"`
		DisplayName string `json:"display_name"`
		Country     string `json:"country"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		badRequest(w, codeInvalidRequest, "請求格式錯誤")
		return
	}

	email := strings.TrimSpace(req.Email)
	if _, err := mail.ParseAddress(email); err != nil {
		badRequest(w, codeEmailInvalid, "email 格式不正確")
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		// 太短和太長是兩件事，前端要能分別講清楚，不能都丟同一個 code。
		code := codePasswordTooLong
		if errors.Is(err, auth.ErrWeakPassword) {
			code = codePasswordTooShort
		}
		badRequest(w, code, err.Error())
		return
	}

	displayName := strings.TrimSpace(req.DisplayName)
	if displayName == "" {
		displayName, _, _ = strings.Cut(email, "@")
	}

	// 所在地是選填。有填的話目錄排序會把在地服務商排前面（見 db/queries/merchants.sql）。
	country, err := geo.NormalizePtr(req.Country)
	if err != nil {
		badRequest(w, codeCountryInvalid, err.Error())
		return
	}

	user, err := s.store.CreateUser(r.Context(), dbgen.CreateUserParams{
		Email:        email,
		DisplayName:  displayName,
		PasswordHash: &hash,
		Country:      country,
	})
	if err != nil {
		if store.IsUniqueViolation(err) {
			conflict(w, codeEmailTaken, "這個 email 已經註冊過了")
			return
		}
		internalError(w, r, err)
		return
	}

	// email 驗證信還沒接，所以帳號建立後 email_verified_at 是空的。
	// 這不影響一般使用，但社群登入要合併同 email 的帳號時會擋下來（見 handleOAuthLogin）。
	s.respondWithTokens(w, r, user)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		badRequest(w, codeInvalidRequest, "請求格式錯誤")
		return
	}

	user, err := s.store.GetUserByEmail(r.Context(), strings.TrimSpace(req.Email))
	if err != nil {
		if store.IsNotFound(err) {
			// 不區分「帳號不存在」和「密碼錯誤」，避免被拿來探測有哪些 email 註冊過。
			unauthorized(w, codeInvalidCredentials, "email 或密碼錯誤")
			return
		}
		internalError(w, r, err)
		return
	}

	if user.PasswordHash == nil {
		unauthorized(w, codeSocialAccountOnly, "這個帳號是用社群帳號註冊的，請改用 Google 或 Apple 登入")
		return
	}
	if !auth.VerifyPassword(*user.PasswordHash, req.Password) {
		unauthorized(w, codeInvalidCredentials, "email 或密碼錯誤")
		return
	}

	s.respondWithTokens(w, r, user)
}

// handleForgotPassword 寄一組驗證碼到信箱。
//
// 不管 email 有沒有註冊過都回 204 —— 回「查無此帳號」等於把這支 endpoint 變成
// 「這個 email 有沒有在站上」的查詢工具，跟 handleLogin 不區分帳號與密碼錯誤是同個理由。
func (s *Server) handleForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
		// 前端當下的介面語言。信是後端直接寄出去的，前端沒有機會翻譯，
		// 所以要在這裡告訴後端該用哪種語言。沒帶就是繁中。
		Locale string `json:"locale"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		badRequest(w, codeInvalidRequest, "請求格式錯誤")
		return
	}

	email := strings.TrimSpace(req.Email)
	if _, err := mail.ParseAddress(email); err != nil {
		badRequest(w, codeEmailInvalid, "email 格式不正確")
		return
	}

	// 在查帳號之前先問，否則 redis 掛掉時「這個 email 沒註冊過」的 204
	// 會把服務不可用蓋掉，使用者只會看到一個永遠等不到的驗證碼。
	if !s.reset.Available() {
		serviceUnavailable(w, codeResetUnavailable, "忘記密碼暫時無法使用，請稍後再試")
		return
	}

	user, err := s.store.GetUserByEmail(r.Context(), email)
	if err != nil {
		if store.IsNotFound(err) {
			writeJSON(w, http.StatusNoContent, nil)
			return
		}
		internalError(w, r, err)
		return
	}

	// 社群帳號也放行：能收到信就證明信箱是本人的，重設等於幫他補一組密碼。
	code, err := s.reset.Issue(r.Context(), email)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrResetTooManyRequests):
			tooManyRequests(w, codeResetTooManyRequests, "索取驗證碼太頻繁，請稍後再試")
		case errors.Is(err, auth.ErrResetUnavailable):
			serviceUnavailable(w, codeResetUnavailable, "忘記密碼暫時無法使用，請稍後再試")
		default:
			internalError(w, r, err)
		}
		return
	}

	subject, body := resetCodeMail(req.Locale, user.DisplayName, code, s.reset.TTL())
	if err := s.mailer.Send(r.Context(), mailer.Message{
		To:      user.Email,
		Subject: subject,
		Body:    body,
	}); err != nil {
		internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

// handleResetPassword 驗證碼對了就換密碼，並且直接發新的 token ——
// 剛設好的密碼再叫使用者手動登入一次是多餘的一步。
func (s *Server) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Code     string `json:"code"`
		Password string `json:"password"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		badRequest(w, codeInvalidRequest, "請求格式錯誤")
		return
	}

	email := strings.TrimSpace(req.Email)
	ctx := r.Context()

	// 先擋掉太短太長，否則使用者會用掉一次驗證碼才被告知密碼不合格。
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		code := codePasswordTooLong
		if errors.Is(err, auth.ErrWeakPassword) {
			code = codePasswordTooShort
		}
		badRequest(w, code, err.Error())
		return
	}

	if err := s.reset.Consume(ctx, email, req.Code); err != nil {
		switch {
		case errors.Is(err, auth.ErrResetCodeExpired):
			badRequest(w, codeResetCodeExpired, "驗證碼已過期，請重新索取")
		case errors.Is(err, auth.ErrResetCodeInvalid):
			badRequest(w, codeResetCodeInvalid, "驗證碼不正確")
		case errors.Is(err, auth.ErrResetTooManyAttempts):
			tooManyRequests(w, codeResetTooManyAttempts, "錯誤次數過多，請重新索取驗證碼")
		case errors.Is(err, auth.ErrResetUnavailable):
			serviceUnavailable(w, codeResetUnavailable, "忘記密碼暫時無法使用，請稍後再試")
		default:
			internalError(w, r, err)
		}
		return
	}

	user, err := s.store.GetUserByEmail(ctx, email)
	if err != nil {
		// 驗證碼發出後帳號才被刪掉，這時候沒有更貼切的說法。
		if store.IsNotFound(err) {
			badRequest(w, codeResetCodeExpired, "驗證碼已過期，請重新索取")
			return
		}
		internalError(w, r, err)
		return
	}

	if err := s.store.UpdateUserPassword(ctx, dbgen.UpdateUserPasswordParams{
		ID:           user.ID,
		PasswordHash: &hash,
	}); err != nil {
		internalError(w, r, err)
		return
	}

	// 會走到忘記密碼，有相當機率是帳號已經被別人登入著。舊的 session 留著就白改了。
	if err := s.store.RevokeAllUserTokens(ctx, user.ID); err != nil {
		internalError(w, r, err)
		return
	}

	// 收得到驗證碼就是證明了信箱所有權，順手把 email 標成已驗證 ——
	// 這也是 handleOAuthLogin 自動合併帳號的前提。
	if user.EmailVerifiedAt == nil {
		if err := s.store.MarkEmailVerified(ctx, user.ID); err != nil {
			internalError(w, r, err)
			return
		}
		now := time.Now()
		user.EmailVerifiedAt = &now
	}

	s.reset.Clear(ctx, email)
	s.respondWithTokens(w, r, user)
}

func (s *Server) handleOAuthLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Provider string `json:"provider"`
		IDToken  string `json:"id_token"`
		// 前端登入頁上選的所在地。已經有帳號的人會被忽略 ——
		// 那是使用者自己在帳號設定裡管的，不該被一次社群登入覆寫。
		Country string `json:"country"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		badRequest(w, codeInvalidRequest, "請求格式錯誤")
		return
	}

	identity, err := s.oidc.Verify(r.Context(), req.Provider, req.IDToken)
	if err != nil {
		unauthorized(w, codeOAuthVerifyFailed, "社群登入驗證失敗")
		return
	}

	ctx := r.Context()

	// 已經綁過就直接登入。
	if link, err := s.store.GetOAuthIdentity(ctx, dbgen.GetOAuthIdentityParams{
		Provider:       identity.Provider,
		ProviderUserID: identity.Subject,
	}); err == nil {
		user, err := s.store.GetUserByID(ctx, link.UserID)
		if err != nil {
			internalError(w, r, err)
			return
		}
		s.respondWithTokens(w, r, user)
		return
	} else if !store.IsNotFound(err) {
		internalError(w, r, err)
		return
	}

	if identity.Email == "" {
		badRequest(w, codeOAuthNoEmail, "這個社群帳號沒有提供 email，無法建立帳號")
		return
	}

	existing, err := s.store.GetUserByEmail(ctx, identity.Email)
	switch {
	case err == nil:
		// 同一個 email 已經有本地帳號。自動合併只在兩邊 email 都驗證過時才做，
		// 否則「用未驗證的 email 註冊 → 對方用社群登入」就變成帳號接管。
		if !identity.EmailVerified || existing.EmailVerifiedAt == nil {
			conflict(w, codeEmailNeedsPasswordLogin, "這個 email 已經有帳號了，請先用密碼登入後再綁定社群帳號")
			return
		}
		if _, err := s.store.CreateOAuthIdentity(ctx, dbgen.CreateOAuthIdentityParams{
			UserID:         existing.ID,
			Provider:       identity.Provider,
			ProviderUserID: identity.Subject,
		}); err != nil {
			internalError(w, r, err)
			return
		}
		s.respondWithTokens(w, r, existing)
		return

	case store.IsNotFound(err):
		// 全新使用者。
	default:
		internalError(w, r, err)
		return
	}

	displayName := identity.Name
	if displayName == "" {
		// Apple 只在第一次授權時給名字，而且不在 id_token 裡，常常是空的。
		displayName, _, _ = strings.Cut(identity.Email, "@")
	}

	var verifiedAt *time.Time
	if identity.EmailVerified {
		now := time.Now()
		verifiedAt = &now
	}
	var avatar *string
	if identity.AvatarURL != "" {
		avatar = &identity.AvatarURL
	}

	country, err := geo.NormalizePtr(req.Country)
	if err != nil {
		badRequest(w, codeCountryInvalid, err.Error())
		return
	}

	user, err := s.store.CreateUser(ctx, dbgen.CreateUserParams{
		Email:           identity.Email,
		DisplayName:     displayName,
		AvatarUrl:       avatar,
		EmailVerifiedAt: verifiedAt,
		Country:         country,
	})
	if err != nil {
		internalError(w, r, err)
		return
	}
	if _, err := s.store.CreateOAuthIdentity(ctx, dbgen.CreateOAuthIdentityParams{
		UserID:         user.ID,
		Provider:       identity.Provider,
		ProviderUserID: identity.Subject,
	}); err != nil {
		internalError(w, r, err)
		return
	}

	s.respondWithTokens(w, r, user)
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		badRequest(w, codeInvalidRequest, "請求格式錯誤")
		return
	}

	pair, err := s.tokens.Refresh(r.Context(), req.RefreshToken, r.UserAgent())
	if err != nil {
		unauthorized(w, codeSessionExpired, "登入已失效，請重新登入")
		return
	}
	writeJSON(w, http.StatusOK, pair)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		badRequest(w, codeInvalidRequest, "請求格式錯誤")
		return
	}
	if err := s.tokens.Revoke(r.Context(), req.RefreshToken); err != nil {
		internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

func (s *Server) handleAdminLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		badRequest(w, codeInvalidRequest, "請求格式錯誤")
		return
	}

	admin, err := s.store.GetAdminByEmail(r.Context(), strings.TrimSpace(req.Email))
	if err != nil {
		if store.IsNotFound(err) {
			unauthorized(w, codeInvalidCredentials, "帳號或密碼錯誤")
			return
		}
		internalError(w, r, err)
		return
	}
	if !auth.VerifyPassword(admin.PasswordHash, req.Password) {
		unauthorized(w, codeInvalidCredentials, "帳號或密碼錯誤")
		return
	}

	// 後台不發 refresh token：session 短、過期就重登，減少長效憑證外流的面。
	token, expiresAt, err := s.tokens.SignAccessToken(admin.ID, auth.SubjectAdmin, admin.Role)
	if err != nil {
		internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": token,
		"expires_at":   expiresAt,
		"admin": map[string]any{
			"id":           admin.ID,
			"email":        admin.Email,
			"display_name": admin.DisplayName,
			"role":         admin.Role,
		},
	})
}

func (s *Server) respondWithTokens(w http.ResponseWriter, r *http.Request, user dbgen.User) {
	pair, err := s.tokens.IssuePair(r.Context(), user.ID, r.UserAgent())
	if err != nil {
		internalError(w, r, err)
		return
	}
	// 登入當下就把 Pro 狀態帶回去，app 才不用為了知道方案再打一次 /v1/me。
	resp := toUserResponse(user)
	resp.IsPro, resp.ProExpiresAt = s.isPro(r, user.ID)

	writeJSON(w, http.StatusOK, authResponse{User: resp, Tokens: pair})
}
