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
	"refcode-api/internal/mailer"
	"refcode-api/internal/ranking"
	"refcode-api/internal/store"
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
}

func NewServer(
	cfg *config.Config,
	st *store.Store,
	tokens *auth.Service,
	oidcVerifier *auth.OIDCVerifier,
	reset *auth.ResetService,
	mail mailer.Mailer,
	images *cloudinary.Client,
) *Server {
	return &Server{
		cfg:      cfg,
		store:    st,
		tokens:   tokens,
		oidc:     oidcVerifier,
		reset:    reset,
		mailer:   mail,
		images:   images,
		rankOpts: ranking.DefaultParams(),
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

			r.Get("/categories", s.handleListCategories)
			r.Get("/categories/{id}", s.handleGetCategory)
			r.Get("/merchants", s.handleListMerchants)
			r.Get("/merchants/sitemap", s.handleMerchantSitemap)
			r.Get("/merchants/{slug}", s.handleGetMerchant)
			r.Post("/events", s.handleCreateEvent)
			r.Post("/codes/{id}/reports", s.handleCreateReport)
		})

		r.Group(func(r chi.Router) {
			r.Use(s.requireUser)

			r.Get("/me", s.handleGetMe)
			r.Patch("/me", s.handleUpdateMe)
			r.Delete("/me", s.handleDeleteMe)
			r.Get("/me/codes", s.handleListMyCodes)
			r.Post("/codes", s.handleCreateCode)
			r.Get("/codes/{id}/stats", s.handleCodeStats)
		})

		r.Route("/admin", func(r chi.Router) {
			r.Post("/login", s.handleAdminLogin)

			r.Group(func(r chi.Router) {
				r.Use(s.requireAdmin)

				r.Get("/codes/pending", s.handleListPendingCodes)
				r.Post("/codes/{id}/review", s.handleReviewCode)

				r.Group(func(r chi.Router) {
					r.Use(requireOwner)

					r.Post("/categories", s.handleCreateCategory)
					r.Patch("/categories/{id}", s.handleUpdateCategory)
					r.Delete("/categories/{id}", s.handleDeleteCategory)
					r.Get("/merchants", s.handleListMerchantsForAdmin)
					r.Post("/merchants", s.handleCreateMerchant)
					r.Patch("/merchants/{id}", s.handleUpdateMerchant)
					r.Post("/uploads/image", s.handleUploadImage)
					r.Get("/users", s.handleAdminListUsers)
					r.Post("/users/{id}/pro", s.handleAdminGrantPro)
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
