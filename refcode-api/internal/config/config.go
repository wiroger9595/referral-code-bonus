package config

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"refcode-api/internal/ranking"
)

type Config struct {
	Env      string
	HTTPAddr string

	DatabaseURL string

	// 忘記密碼的驗證碼與寄送次數限制放 redis：兩者都是短命且到期就該消失的東西，
	// 進 Postgres 只會多一張需要自己清的表。
	RedisAddr     string
	RedisPassword string
	RedisDB       int
	RedisTLS      bool

	// 寄信。SMTPHost 留空時不會真的寄出去，改成把信印進 log（見 mailer），
	// 本機開發不用先架一台 SMTP；正式環境兩個都留空會在啟動時擋下來。
	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	MailFrom     string
	MailFromName string

	// 設了的話優先於 SMTP（見 mailer.New）。
	ResendAPIKey string

	JWTSecret       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration

	// 忘記密碼。TTL 短是因為 6 位數只有一百萬種組合，能猜的時間窗要壓小；
	// MaxAttempts 是同一組碼可以猜錯幾次，MaxSends 是每小時能要幾次新碼。
	PasswordResetTTL         time.Duration
	PasswordResetMaxAttempts int
	PasswordResetMaxSends    int

	// 三個前端各自的 origin，開發時是 localhost 的三個 port。
	CORSOrigins []string

	// 社群登入的 audience。Apple 的 client id 在 app 端是 bundle id、
	// 在 web 端是 Services ID，兩個都要收，所以是列表。
	GoogleClientIDs []string
	AppleClientIDs  []string

	// 回報統計的自動下架門檻，調參用，不寫死在邏輯裡。
	AutoDisableMinReports int
	AutoDisableFailRatio  float64

	// 排序引擎的調參。預設值定義在 internal/ranking，這裡只是讓正式環境
	// 不必重新 build 就能調——這幾個是商業參數，會反覆試。
	Ranking ranking.Params

	// 訂閱。真相在 RevenueCat，這裡只收 webhook 並保存一份副本供伺服器端判斷。
	// WebhookAuth 是 RevenueCat 後台設定的 Authorization 標頭值，原樣比對；
	// 留空代表不開放 webhook（那支路由會直接回 404，而不是無驗證地收單）。
	RevenueCatWebhookAuth string
	ProEntitlement        string
	FreeActiveCodeLimit   int

	// 後台上傳圖片（服務商 logo、分類圖）存去 Cloudinary。留空就是沒設定，
	// 上傳端點會直接回錯誤，不影響其他功能。
	CloudinaryCloudName string
	CloudinaryAPIKey    string
	CloudinaryAPISecret string
}

func Load() (*Config, error) {
	loadDotEnv(".env")

	cfg := &Config{
		Env:      env("APP_ENV", "development"),
		HTTPAddr: env("HTTP_ADDR", ":7802"),

		DatabaseURL: env("DATABASE_URL", ""),

		RedisAddr:     net.JoinHostPort(env("REDIS_HOST", "127.0.0.1"), env("REDIS_PORT", "6379")),
		RedisPassword: env("REDIS_PASSWORD", ""),
		RedisDB:       envInt("REDIS_DB", 0),
		RedisTLS:      envBool("REDIS_TLS", false),

		SMTPHost:     env("SMTP_HOST", ""),
		SMTPPort:     envInt("SMTP_PORT", 587),
		SMTPUsername: env("SMTP_USERNAME", ""),
		SMTPPassword: env("SMTP_PASSWORD", ""),
		MailFrom:     env("MAIL_FROM", "no-reply@localhost"),
		MailFromName: env("MAIL_FROM_NAME", "推薦碼交流站"),
		ResendAPIKey: env("RESEND_API_KEY", ""),

		JWTSecret:       env("JWT_SECRET", ""),
		AccessTokenTTL:  envDuration("ACCESS_TOKEN_TTL", 15*time.Minute),
		RefreshTokenTTL: envDuration("REFRESH_TOKEN_TTL", 30*24*time.Hour),

		CORSOrigins:     envList("CORS_ORIGINS", "http://localhost:3000,http://localhost:5173,http://localhost:5174"),
		GoogleClientIDs: envList("GOOGLE_CLIENT_IDS", ""),
		AppleClientIDs:  envList("APPLE_CLIENT_IDS", ""),

		PasswordResetTTL:         envDuration("PASSWORD_RESET_TTL", 10*time.Minute),
		PasswordResetMaxAttempts: envInt("PASSWORD_RESET_MAX_ATTEMPTS", 5),
		PasswordResetMaxSends:    envInt("PASSWORD_RESET_MAX_SENDS", 5),

		AutoDisableMinReports: envInt("AUTO_DISABLE_MIN_REPORTS", 3),
		AutoDisableFailRatio:  envFloat("AUTO_DISABLE_FAIL_RATIO", 0.6),

		Ranking: loadRanking(),

		RevenueCatWebhookAuth: env("REVENUECAT_WEBHOOK_AUTH", ""),
		ProEntitlement:        env("PRO_ENTITLEMENT", "pro"),
		FreeActiveCodeLimit:   envInt("FREE_ACTIVE_CODE_LIMIT", 3),

		CloudinaryCloudName: env("CLOUDINARY_CLOUD_NAME", ""),
		CloudinaryAPIKey:    env("CLOUDINARY_API_KEY", ""),
		CloudinaryAPISecret: env("CLOUDINARY_API_SECRET", ""),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL 未設定")
	}
	if len(cfg.JWTSecret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET 至少要 32 個字元（目前 %d）", len(cfg.JWTSecret))
	}
	// 正式環境兩個寄信管道都沒設的話，忘記密碼會安靜地把驗證碼印進 log 而不是寄出去 ——
	// 那比直接壞掉更糟，寧可啟動時就擋下來。
	if cfg.IsProduction() && cfg.SMTPHost == "" && cfg.ResendAPIKey == "" {
		return nil, fmt.Errorf("SMTP_HOST 或 RESEND_API_KEY 至少要設一個，正式環境不能不寄信")
	}
	// 這兩個填錯不會噴錯，只會讓權重靜靜地算出 Inf 或負數，整個列表順序爛掉還查不出原因。
	if cfg.Ranking.ImpressionSoftCap <= 0 {
		return nil, fmt.Errorf("RANK_IMPRESSION_SOFT_CAP 必須大於 0（目前 %v）", cfg.Ranking.ImpressionSoftCap)
	}
	if cfg.Ranking.FreshnessBoost < 0 {
		return nil, fmt.Errorf("RANK_FRESHNESS_BOOST 不能是負數（目前 %v）", cfg.Ranking.FreshnessBoost)
	}
	return cfg, nil
}

func (c *Config) IsProduction() bool { return c.Env == "production" }

// 已存在的環境變數優先，.env 只當本機開發的預設值。
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.Trim(strings.TrimSpace(v), `"'`)
		if _, exists := os.LookupEnv(k); !exists {
			os.Setenv(k, v)
		}
	}
}

// loadRanking 以 internal/ranking 的預設值為底，逐項讓 env 覆蓋。
// 預設值刻意只留一份在演算法旁邊——分兩處寫早晚會漂移，
// 而排序參數漂掉的症狀（列表順序怪怪的）不會有人立刻發現。
func loadRanking() ranking.Params {
	p := ranking.DefaultParams()
	return ranking.Params{
		FreshnessBoost:    envFloat("RANK_FRESHNESS_BOOST", p.FreshnessBoost),
		FreeGraceHours:    envFloat("RANK_FREE_GRACE_HOURS", p.FreeGraceHours),
		FreeDecayHours:    envFloat("RANK_FREE_DECAY_HOURS", p.FreeDecayHours),
		ProGraceHours:     envFloat("RANK_PRO_GRACE_HOURS", p.ProGraceHours),
		ProDecayHours:     envFloat("RANK_PRO_DECAY_HOURS", p.ProDecayHours),
		ImpressionSoftCap: envFloat("RANK_IMPRESSION_SOFT_CAP", p.ImpressionSoftCap),
		ProBoost:          envFloat("RANK_PRO_BOOST", p.ProBoost),
	}
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envList(key, def string) []string {
	raw := env(key, def)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func envDuration(key string, def time.Duration) time.Duration {
	if v, err := time.ParseDuration(env(key, "")); err == nil {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v, err := strconv.Atoi(env(key, "")); err == nil {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	if v, err := strconv.ParseBool(env(key, "")); err == nil {
		return v
	}
	return def
}

func envFloat(key string, def float64) float64 {
	if v, err := strconv.ParseFloat(env(key, ""), 64); err == nil {
		return v
	}
	return def
}
