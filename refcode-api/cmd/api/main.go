package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"refcode-api/internal/auth"
	"refcode-api/internal/cloudinary"
	"refcode-api/internal/config"
	"refcode-api/internal/httpapi"
	"refcode-api/internal/kv"
	"refcode-api/internal/mailer"
	"refcode-api/internal/store"
	"refcode-api/internal/worker"
)

func main() {
	if err := run(); err != nil {
		slog.Error("啟動失敗", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 撞埠是本機開發最常見的啟動失敗，所以先把 port 佔下來再連資料庫。
	// 反過來的話 bind 失敗會取消 ctx，worker 正在做的查詢跟著被取消，
	// log 上會多一行看起來像「資料庫連不上」的假線索。
	ln, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		if errors.Is(err, syscall.EADDRINUSE) {
			return fmt.Errorf("port %s 已經被%s佔用。停掉它：%s", cfg.HTTPAddr, portHolder(cfg.HTTPAddr), killHint(cfg.HTTPAddr))
		}
		return err
	}
	defer ln.Close()

	st, err := store.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer st.Close()

	// redis 只有忘記密碼在用。連不上就讓那兩支路由回 503，其他功能照常 ——
	// 為了一個附屬流程讓整個 API 起不來不划算。
	rdb, err := kv.New(ctx, kv.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
		UseTLS:   cfg.RedisTLS,
	})
	if err != nil {
		slog.Error("redis 連線失敗，忘記密碼暫時停用", "err", err)
	} else {
		defer rdb.Close()
	}

	tokens := auth.NewService(st, cfg.JWTSecret, cfg.AccessTokenTTL, cfg.RefreshTokenTTL)
	oidcVerifier := auth.NewOIDCVerifier(cfg.GoogleClientIDs, cfg.AppleClientIDs)
	reset := auth.NewResetService(rdb, cfg.JWTSecret, cfg.PasswordResetTTL, cfg.PasswordResetMaxAttempts, cfg.PasswordResetMaxSends)
	mail := mailer.New(mailer.Config{
		Host:         cfg.SMTPHost,
		Port:         cfg.SMTPPort,
		Username:     cfg.SMTPUsername,
		Password:     cfg.SMTPPassword,
		From:         cfg.MailFrom,
		FromName:     cfg.MailFromName,
		ResendAPIKey: cfg.ResendAPIKey,
	})

	images := cloudinary.New(cfg.CloudinaryCloudName, cfg.CloudinaryAPIKey, cfg.CloudinaryAPISecret)
	if !images.Enabled() {
		slog.Warn("CLOUDINARY_* 未設定，後台圖片上傳暫時停用")
	}

	go worker.New(st).Run(ctx)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           httpapi.NewServer(cfg, st, tokens, oidcVerifier, reset, mail, images).Routes(),
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("API 啟動", "addr", cfg.HTTPAddr, "env", cfg.Env)
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	// 收到訊號後給進行中的請求 15 秒收尾。
	slog.Info("收到關閉訊號，正在停止")

	// srv.Shutdown 逾時只代表它自己放棄等待，不會強制斷開還沒還回 pool 的
	// DB 連線；接下來的 defer st.Close()/rdb.Close() 沒有 timeout，卡住的話
	// process 永遠不退出、port 也永遠放不掉。這裡設一個硬上限兜底。
	go func() {
		time.Sleep(20 * time.Second)
		slog.Error("關閉流程逾時，強制結束")
		os.Exit(1)
	}()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

// portHolder 回傳佔著這個 port 的 pid 與執行檔，查不到就回空字串——只是為了讓
// 撞埠的錯誤訊息好懂，查不到也不影響結果。
//
// 執行檔名要一起印：`go run` 真正 listen 的是編在 /var/folders 底下的暫存檔，
// 只給 pid 的話看不出那是自己剛才那支 api 還是別的程式。
func portHolder(addr string) string {
	pids := portPIDs(addr)
	if len(pids) == 0 {
		return ""
	}

	out, err := exec.Command("ps", "-o", "comm=", "-p", strings.Join(pids, ",")).Output()
	names := strings.Fields(string(out))
	if err != nil || len(names) == 0 {
		return " pid " + strings.Join(pids, "/") + " "
	}
	return fmt.Sprintf(" pid %s（%s）", strings.Join(pids, "/"), strings.Join(names, " "))
}

// killHint 給一句在任何工作目錄下都能直接貼進 terminal 的指令。
//
// 這裡刻意不寫「跑 ./dev.sh stop api」——`go run ./cmd/api` 的工作目錄是
// refcode-api/，而 dev.sh 在 workspace 根目錄，那個相對路徑在這個情境下必定
// 找不到檔案，看起來就像「照著做了但停不掉」。
func killHint(addr string) string {
	_, port, err := net.SplitHostPort(addr)
	if err != nil || port == "" {
		return "lsof -ti tcp:<port> | xargs kill"
	}
	return fmt.Sprintf("lsof -ti tcp:%s | xargs kill", port)
}

func portPIDs(addr string) []string {
	_, port, err := net.SplitHostPort(addr)
	if err != nil || port == "" {
		return nil
	}
	out, err := exec.Command("lsof", "-ti:"+port).Output()
	if err != nil {
		return nil
	}
	return strings.Fields(string(out))
}
