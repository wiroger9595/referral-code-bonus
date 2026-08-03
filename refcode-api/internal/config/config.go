package config

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
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
	// 本機開發不用先架一台 SMTP；正式環境留空會在啟動時擋下來。
	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	MailFrom     string
	MailFromName string

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

	// 訂閱。真相在 RevenueCat，這裡只收 webhook 並保存一份副本供伺服器端判斷。
	// WebhookAuth 是 RevenueCat 後台設定的 Authorization 標頭值，原樣比對；
	// 留空代表不開放 webhook（那支路由會直接回 404，而不是無驗證地收單）。
	RevenueCatWebhookAuth string
	ProEntitlement        string
	FreeActiveCodeLimit   int
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

		RevenueCatWebhookAuth: env("REVENUECAT_WEBHOOK_AUTH", ""),
		ProEntitlement:        env("PRO_ENTITLEMENT", "pro"),
		FreeActiveCodeLimit:   envInt("FREE_ACTIVE_CODE_LIMIT", 3),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL 未設定")
	}
	if len(cfg.JWTSecret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET 至少要 32 個字元（目前 %d）", len(cfg.JWTSecret))
	}
	// 正式環境沒設 SMTP 的話，忘記密碼會安靜地把驗證碼印進 log 而不是寄出去 ——
	// 那比直接壞掉更糟，寧可啟動時就擋下來。
	if cfg.IsProduction() && cfg.SMTPHost == "" {
		return nil, fmt.Errorf("SMTP_HOST 未設定，正式環境不能不寄信")
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
