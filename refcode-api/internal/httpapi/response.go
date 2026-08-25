package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
)

type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// code 是前端唯一可靠的判斷依據 —— message 是中文，日文與英文介面沒辦法直接顯示，
// 它們要拿 code 去查自己的語系檔。所以每個錯誤都要有能對應到單一句子的 code，
// 不要幾十種情況共用一個 conflict。新增錯誤時一起在這裡加一個常數。
//
// 泛用的幾個：只有在真的沒有更精確的說法時才用（例如 body 根本不是合法 JSON）。
const (
	codeInvalidRequest = "invalid_request"
	codeUnauthorized   = "unauthorized"
	codeForbidden      = "forbidden"
	codeNotFound       = "not_found"
	codeConflict       = "conflict"
	codeInternal       = "internal_error"
)

// 認證。
const (
	codeEmailInvalid            = "email_invalid"
	codeEmailTaken              = "email_taken"
	codePasswordTooShort        = "password_too_short"
	codePasswordTooLong         = "password_too_long"
	codeInvalidCredentials      = "invalid_credentials"
	codeSocialAccountOnly       = "social_account_only"
	codeOAuthVerifyFailed       = "oauth_verify_failed"
	codeOAuthNoEmail            = "oauth_no_email"
	codeEmailNeedsPasswordLogin = "email_needs_password_login"
	codeSessionExpired          = "session_expired"
	codeResetCodeExpired        = "reset_code_expired"
	codeResetCodeInvalid        = "reset_code_invalid"
	codeResetTooManyAttempts    = "reset_too_many_attempts"
	codeResetTooManyRequests    = "reset_too_many_requests"
	codeResetUnavailable        = "reset_unavailable"
	codeLoginRequired           = "login_required"
	codeAdminRequired           = "admin_required"
	codeOwnerRequired           = "owner_required"
)

// 上架與瀏覽。
const (
	codeInvalidID           = "invalid_id"
	codeDisplayNameRequired = "display_name_required"
	codeCountryInvalid      = "country_invalid"
	codeMerchantNotFound    = "merchant_not_found"
	codeMerchantClosed      = "merchant_closed"
	codeCodeRequired        = "code_required"
	codeCodeFormatMismatch  = "code_format_mismatch"
	codeCodeTypeInvalid     = "code_type_invalid"
	// 這家服務商沒開放這種碼（例如沒有推薦計畫的服務商只收折扣碼）。
	codeCodeTypeNotAllowed = "code_type_not_allowed"
	// 折扣碼沒有結構化的優惠欄位，備註就是唯一說明優惠內容的地方。
	codeDiscountNoteRequired = "discount_note_required"
	codeExpiryInPast         = "expiry_in_past"
	codeExpiryTooFar         = "expiry_too_far"
	codeCodeAlreadyListed    = "code_already_listed"
	codeCodeNotFound         = "code_not_found"
	codeCodeNotActive        = "code_not_active"
	codeEventTypeInvalid     = "event_type_invalid"
	codeReportResultInvalid  = "report_result_invalid"
	codeCannotBlockSelf      = "cannot_block_self"
	codeProRequired          = "pro_required"
	codeDeleteConfirmation   = "delete_confirmation_mismatch"
	// 使用者提報希望上架的平台（見 handleCreateMerchantSuggestion）。
	codeSuggestionNameRequired   = "suggestion_name_required"
	codeSuggestionNameTooLong    = "suggestion_name_too_long"
	codeSuggestionNoteTooLong    = "suggestion_note_too_long"
	codeSuggestionURLInvalid     = "suggestion_url_invalid"
	codeSuggestionDuplicate      = "suggestion_duplicate"
	codeSuggestionMerchantExists = "suggestion_merchant_exists"
	codeSuggestionLimitReached   = "suggestion_limit_reached"
)

// 後台。這幾個只有 refcode-admin 會遇到，admin 是單一語言，不需要翻譯。
const (
	codeReviewActionInvalid  = "review_action_invalid"
	codeReviewReasonRequired = "review_reason_required"
	codeCodeStatusInvalid    = "code_status_invalid"
	codeSlugInvalid          = "slug_invalid"
	codeSlugTaken            = "slug_taken"
	codeNameRequired         = "name_required"
	codeCategoryNotFound     = "category_not_found"
	codeCategoryInUse        = "category_in_use"
	codeImageInvalid         = "image_invalid"
	codeUserNotFound         = "user_not_found"
	codeSuggestionNotFound   = "suggestion_not_found"
	codeSuggestionReviewed   = "suggestion_already_reviewed"
)

// 基礎設施與 webhook。前兩個是機器對機器，不會顯示給使用者。
const (
	codeWebhookAuthFailed = "webhook_auth_failed"
	codeMissingEventID    = "missing_event_id"
	codeDatabaseDown      = "database_unavailable"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("寫入 response 失敗", "err", err)
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorBody{Error: errorDetail{Code: code, Message: message}})
}

func badRequest(w http.ResponseWriter, code, message string) {
	writeError(w, http.StatusBadRequest, code, message)
}

func unauthorized(w http.ResponseWriter, code, message string) {
	writeError(w, http.StatusUnauthorized, code, message)
}

func forbidden(w http.ResponseWriter, code, message string) {
	writeError(w, http.StatusForbidden, code, message)
}

func notFound(w http.ResponseWriter, code, message string) {
	writeError(w, http.StatusNotFound, code, message)
}

func conflict(w http.ResponseWriter, code, message string) {
	writeError(w, http.StatusConflict, code, message)
}

func tooManyRequests(w http.ResponseWriter, code, message string) {
	writeError(w, http.StatusTooManyRequests, code, message)
}

func serviceUnavailable(w http.ResponseWriter, code, message string) {
	writeError(w, http.StatusServiceUnavailable, code, message)
}

// internalError 對外只給一句話，細節進 log —— 不要把 DB 錯誤丟給前端。
func internalError(w http.ResponseWriter, r *http.Request, err error) {
	slog.Error("處理請求失敗",
		"method", r.Method,
		"path", r.URL.Path,
		"err", err,
	)
	writeError(w, http.StatusInternalServerError, codeInternal, "伺服器發生錯誤，請稍後再試")
}

// decodeJSON 限制 1MB 並拒絕未知欄位，避免前端拼錯欄位名卻靜靜地被忽略。
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body 只能有一個 JSON 物件")
	}
	return nil
}
