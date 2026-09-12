package httpapi

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"refcode-api/internal/auth"
	"refcode-api/internal/cloudinary"
	"refcode-api/internal/config"
	"refcode-api/internal/entitlement"
	"refcode-api/internal/mailer"
	"refcode-api/internal/ranking"
	"refcode-api/internal/store"
	"refcode-api/internal/suspension"
	"refcode-api/internal/worker"
)

type Server struct {
	cfg      *config.Config
	store    *store.Store
	tokens   *auth.Service
	oidc     *auth.OIDCVerifier
	reset    *auth.ResetService
	mailer   mailer.Mailer
	images   *cloudinary.Client
	rankOpts ranking.Params
	// 訂閱狀態變動後把架上的碼收斂回該有的張數。跟 worker 用的是同一份邏輯，
	// 只是各自持有一個 —— Syncer 沒有狀態，不值得為它多拉一條建構參數。
	ent *entitlement.Syncer
	// 停權與解除要連帶處理架上的碼與 refresh token。跟 ent 同理，worker 也持有
	// 一個各自的 —— Manager 沒有狀態。
	susp *suspension.Manager
	// 後台的排程頁要顯示每支 job 的說明，那只有程式碼這邊有（資料庫那張表
	// 只存開關與間隔）。存的是註冊清單本身，不是 *worker.Worker —— API 不該
	// 有辦法直接叫排程跑起來，那是 worker 自己輪詢認領的事。
	jobs []worker.Job
}

func NewServer(
	cfg *config.Config,
	st *store.Store,
	tokens *auth.Service,
	oidcVerifier *auth.OIDCVerifier,
	reset *auth.ResetService,
	mail mailer.Mailer,
	images *cloudinary.Client,
	jobs []worker.Job,
) *Server {
	return &Server{
		cfg:      cfg,
		store:    st,
		tokens:   tokens,
		oidc:     oidcVerifier,
		reset:    reset,
		mailer:   mail,
		images:   images,
		rankOpts: cfg.Ranking,
		ent:      entitlement.New(st, cfg.FreeActiveCodeLimit),
		susp:     suspension.New(st),
		jobs:     jobs,
	}
}

func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.RealIP)
	r.Use(chimw.Recoverer)
	r.Use(requestLogger)
	r.Use(chimw.Timeout(30 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   s.cfg.CORSOrigins,
		AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "X-Device-ID"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	r.Get("/healthz", s.handleHealth)

	r.Route("/v1", func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", s.handleRegister)
			r.Post("/login", s.handleLogin)
			r.Post("/oauth", s.handleOAuthLogin)
			r.Post("/refresh", s.handleRefresh)
			r.Post("/logout", s.handleLogout)
			r.Post("/password/forgot", s.handleForgotPassword)
			r.Post("/password/reset", s.handleResetPassword)
		})

		// RevenueCat 的訂閱事件。不走 requireUser —— 來的是 RevenueCat 的伺服器，
		// 不是使用者，驗證用的是設定裡的共用密鑰（見 handler）。
		r.Post("/webhooks/revenuecat", s.handleRevenueCatWebhook)

		// 瀏覽類：匿名可用，帶了 token 就順便歸戶。
		r.Group(func(r chi.Router) {
			r.Use(s.optionalUser)

			r.Get("/regions", s.handleListRegions)
			r.Get("/categories", s.handleListCategories)
			r.Get("/categories/{id}", s.handleGetCategory)
			r.Get("/merchants", s.handleListMerchants)
			r.Get("/merchants/sitemap", s.handleMerchantSitemap)
			r.Get("/merchants/{slug}", s.handleGetMerchant)
			r.Get("/search/popular", s.handleSearchPopular)
			r.Post("/events", s.handleCreateEvent)
			r.Post("/codes/{id}/reports", s.handleCreateReport)
		})

		r.Group(func(r chi.Router) {
			r.Use(s.requireUser)

			r.Get("/me", s.handleGetMe)
			r.Patch("/me", s.handleUpdateMe)
			r.Delete("/me", s.handleDeleteMe)
			r.Post("/me/avatar", s.handleUploadAvatar)
			r.Get("/me/blocks", s.handleListMyBlocks)
			r.Delete("/me/blocks/{id}", s.handleUnblockUser)
			r.Post("/codes/{id}/block-owner", s.handleBlockCodeOwner)
			r.Get("/me/codes", s.handleListMyCodes)
			r.Post("/codes", s.handleCreateCode)
			// 提報希望上架的平台。要登入 —— 建議單會進人工審核佇列，
			// 匿名放行等於開一個沒有成本的洗版管道。
			r.Post("/merchant-suggestions", s.handleCreateMerchantSuggestion)
			r.Post("/codes/{id}/disable", s.handleDisableMyCode)
			r.Get("/codes/{id}/stats", s.handleCodeStats)
		})

		r.Route("/admin", func(r chi.Router) {
			r.Post("/login", s.handleAdminLogin)

			r.Group(func(r chi.Router) {
				r.Use(s.requireAdmin)

				r.Get("/codes", s.handleAdminListCodes)
				r.Get("/codes/pending", s.handleListPendingCodes)
				r.Get("/codes/auto-disabled", s.handleListAutoDisabledCodes)
				r.Post("/codes/{id}/review", s.handleReviewCode)

				r.Group(func(r chi.Router) {
					r.Use(requireOwner)

					r.Post("/categories", s.handleCreateCategory)
					r.Patch("/categories/{id}", s.handleUpdateCategory)
					r.Delete("/categories/{id}", s.handleDeleteCategory)
					// 平台建議放在 owner 這一段而不是審核佇列：通過等於建立一家
					// 服務商，那本來就只有 owner 能做（見下面的 /merchants）。
					r.Get("/merchant-suggestions", s.handleListMerchantSuggestions)
					r.Post("/merchant-suggestions/{id}/review", s.handleReviewMerchantSuggestion)
					r.Get("/merchants", s.handleListMerchantsForAdmin)
					// 稽核判定連續兩次找不到推薦碼字樣的，等人工複檢。
					// 靜態路徑要排在 /merchants/{id} 前面。
					r.Get("/merchants/code-audit", s.handleListMerchantsFlaggedByCodeAudit)
					r.Post("/merchants", s.handleCreateMerchant)
					r.Patch("/merchants/{id}", s.handleUpdateMerchant)
					r.Post("/uploads/image", s.handleUploadImage)
					// 排程：開關、改間隔、手動觸發、看執行紀錄。放在 owner 這一段
					// —— 這些按鈕會去打外站、寫目錄資料，不是審核人員該碰的。
					r.Get("/jobs", s.handleListJobs)
					r.Patch("/jobs/{name}", s.handleUpdateJob)
					r.Post("/jobs/{name}/run", s.handleRunJob)
					r.Get("/jobs/{name}/runs", s.handleListJobRuns)
					r.Get("/users", s.handleAdminListUsers)
					r.Post("/users/{id}/pro", s.handleAdminGrantPro)
					// 停權與解除停權。放在 owner 這一段 —— 這會把一個人的碼
					// 全部從目錄上撤下來，跟建服務商同一個量級的決定。
					r.Post("/users/{id}/suspend", s.handleAdminSuspendUser)
					r.Delete("/users/{id}/suspend", s.handleAdminReinstateUser)
					r.Delete("/users/{id}/pro", s.handleAdminRevokePro)
				})
			})
		})
	})

	return r
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if err := s.store.Pool.Ping(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, codeDatabaseDown, "資料庫連線異常")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
