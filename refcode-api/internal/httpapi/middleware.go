package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"refcode-api/internal/auth"
	"refcode-api/internal/store"
	"refcode-api/internal/store/dbgen"
)

// currentUser 取出目前登入的使用者。
//
// requireUser 只驗 token 的簽章，不查資料庫 —— 所以帳號被刪掉之後，那張還沒過期的
// access token（預設 15 分鐘）依然通得過 middleware。這裡把「token 有效但人不在了」
// 對應成 401，app 收到就會清掉 token 導回登入頁；回 500 的話使用者會卡在錯誤畫面。
func (s *Server) currentUser(w http.ResponseWriter, r *http.Request) (dbgen.User, bool) {
	userID, _ := auth.UserID(r.Context())

	user, err := s.store.GetUserByID(r.Context(), userID)
	if err != nil {
		if store.IsNotFound(err) {
			unauthorized(w, codeSessionExpired, "登入階段已失效，請重新登入")
			return dbgen.User{}, false
		}
		internalError(w, r, err)
		return dbgen.User{}, false
	}
	return user, true
}

// requireUser 擋下未登入的請求。瀏覽類 API 不掛這層 —— 匿名也要能看碼。
func (s *Server) requireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := s.parseBearer(r)
		if !ok || claims.Subject != auth.SubjectUser {
			unauthorized(w, codeLoginRequired, "請先登入")
			return
		}
		id, err := uuid.Parse(claims.RegisteredClaims.Subject)
		if err != nil {
			unauthorized(w, codeLoginRequired, "請先登入")
			return
		}
		next.ServeHTTP(w, r.WithContext(auth.WithUserID(r.Context(), id)))
	})
}

// optionalUser 有帶 token 就解析，沒帶也放行。
// 用在瀏覽類 API：登入者的行為要能歸戶，匿名訪客同樣要能使用。
func (s *Server) optionalUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if claims, ok := s.parseBearer(r); ok && claims.Subject == auth.SubjectUser {
			if id, err := uuid.Parse(claims.RegisteredClaims.Subject); err == nil {
				ctx = auth.WithUserID(ctx, id)
			}
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := s.parseBearer(r)
		if !ok || claims.Subject != auth.SubjectAdmin {
			unauthorized(w, codeAdminRequired, "需要管理員權限")
			return
		}
		id, err := uuid.Parse(claims.RegisteredClaims.Subject)
		if err != nil {
			unauthorized(w, codeAdminRequired, "需要管理員權限")
			return
		}
		ctx := auth.WithAdmin(r.Context(), auth.AdminInfo{ID: id, Role: claims.Role})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// requireOwner 是 admin 之上的一層：只有 owner 能改服務商目錄，
// reviewer 只能審碼。
func requireOwner(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info, ok := auth.Admin(r.Context())
		if !ok || info.Role != "owner" {
			forbidden(w, codeOwnerRequired, "只有 owner 能執行這個操作")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) parseBearer(r *http.Request) (*auth.Claims, bool) {
	raw := r.Header.Get("Authorization")
	if !strings.HasPrefix(raw, "Bearer ") {
		return nil, false
	}
	claims, err := s.tokens.ParseAccessToken(strings.TrimPrefix(raw, "Bearer "))
	if err != nil {
		return nil, false
	}
	return claims, true
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"ms", time.Since(start).Milliseconds(),
		)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// deviceHash 是匿名去重的依據：回報一次只能一次、Phase 3 的點擊計費也靠它。
// client 會帶一個安裝時產生的 X-Device-ID；沒帶的話退回 IP + UA，
// 精準度較差但至少擋得住最直接的重複提交。
func deviceHash(r *http.Request) string {
	id := r.Header.Get("X-Device-ID")
	if id == "" {
		id = clientIP(r) + "|" + r.UserAgent()
	}
	sum := sha256.Sum256([]byte("device:" + id))
	return hex.EncodeToString(sum[:])
}

func ipHash(r *http.Request) string {
	sum := sha256.Sum256([]byte("ip:" + clientIP(r)))
	return hex.EncodeToString(sum[:])
}

// 官網是 SSR 的，搜尋引擎每爬一次服務商頁就會產生一整批曝光。
// 那會直接稀釋排序用的曝光懲罰，讓權重失真，所以爬蟲的曝光不記。
var botUAMarkers = []string{
	"bot", "crawler", "spider", "slurp", "facebookexternalhit",
	"embedly", "quora link preview", "headlesschrome", "python-requests",
	"curl/", "wget/", "go-http-client",
}

func isBot(r *http.Request) bool {
	ua := strings.ToLower(r.UserAgent())
	if ua == "" {
		return true
	}
	for _, marker := range botUAMarkers {
		if strings.Contains(ua, marker) {
			return true
		}
	}
	return false
}

func clientIP(r *http.Request) string {
	// 正式環境一定在反向代理後面，取 X-Forwarded-For 的第一段。
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if first, _, found := strings.Cut(xff, ","); found {
			return strings.TrimSpace(first)
		}
		return strings.TrimSpace(xff)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
