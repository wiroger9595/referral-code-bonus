package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	ErrResetCodeExpired     = errors.New("驗證碼已過期")
	ErrResetCodeInvalid     = errors.New("驗證碼不正確")
	ErrResetTooManyAttempts = errors.New("驗證碼猜錯太多次")
	ErrResetTooManyRequests = errors.New("索取驗證碼太頻繁")
	ErrResetUnavailable     = errors.New("redis 未連線，忘記密碼停用")
)

// ResetService 管忘記密碼的驗證碼。狀態全部在 redis，重啟 API 不會留下孤兒資料。
//
// 6 位數只有一百萬種組合，光靠雜湊擋不住暴力猜，所以三道限制缺一不可：
// TTL 短、同一組碼能猜錯的次數有限、每小時能要幾組新碼也有限。
//
// rdb 可以是 nil —— 連不上 redis 時 API 照常啟動，只有忘記密碼這條路回 503
// （見 cmd/api/main.go）。站上其他功能都不需要 redis，不值得為它讓整個服務起不來。
type ResetService struct {
	rdb         *redis.Client
	secret      []byte
	ttl         time.Duration
	maxAttempts int
	maxSends    int
}

func NewResetService(rdb *redis.Client, secret string, ttl time.Duration, maxAttempts, maxSends int) *ResetService {
	return &ResetService{
		rdb:         rdb,
		secret:      []byte(secret),
		ttl:         ttl,
		maxAttempts: maxAttempts,
		maxSends:    maxSends,
	}
}

func (s *ResetService) TTL() time.Duration { return s.ttl }

// Available 回報這個流程現在能不能用。呼叫端要在查帳號之前先問 ——
// redis 掛掉是服務問題，不該被「不透露 email 有沒有註冊過」那段邏輯蓋成 204。
func (s *ResetService) Available() bool { return s.rdb != nil }

// Issue 產一組新驗證碼並回傳明碼給呼叫端寄出去。redis 只留 HMAC，
// 這樣就算 redis 的內容外流，沒有伺服器密鑰也無法把碼還原（單純 sha256 的話
// 一百萬種組合幾秒就跑完了）。
//
// 同一個 email 重複索取會直接覆蓋掉前一組 —— 舊碼立刻失效，避免同時有多組碼能用。
func (s *ResetService) Issue(ctx context.Context, email string) (string, error) {
	if s.rdb == nil {
		return "", ErrResetUnavailable
	}
	key := emailKey(email)

	sends, err := s.rdb.Incr(ctx, "pwreset:sends:"+key).Result()
	if err != nil {
		return "", err
	}
	if sends == 1 {
		// 只有第一次要設到期，之後設會把計數視窗一直往後推。
		if err := s.rdb.Expire(ctx, "pwreset:sends:"+key, time.Hour).Err(); err != nil {
			return "", err
		}
	}
	if int(sends) > s.maxSends {
		return "", ErrResetTooManyRequests
	}

	code, err := randomCode()
	if err != nil {
		return "", err
	}

	codeKey := "pwreset:code:" + key
	pipe := s.rdb.TxPipeline()
	pipe.Del(ctx, codeKey)
	pipe.HSet(ctx, codeKey, "hash", s.hashCode(key, code), "attempts", 0)
	pipe.Expire(ctx, codeKey, s.ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return "", err
	}
	return code, nil
}

// Consume 驗證碼並在成功時立刻作廢，同一組碼不能用第二次。
func (s *ResetService) Consume(ctx context.Context, email, code string) error {
	if s.rdb == nil {
		return ErrResetUnavailable
	}
	key := emailKey(email)
	codeKey := "pwreset:code:" + key

	stored, err := s.rdb.HGet(ctx, codeKey, "hash").Result()
	if errors.Is(err, redis.Nil) {
		// 沒發過、已過期、或已經用掉了，對使用者來說都是「再要一次新的」。
		return ErrResetCodeExpired
	}
	if err != nil {
		return err
	}

	// 先記猜錯次數再比對，否則猜錯的人只要中途斷線就能把次數洗掉。
	attempts, err := s.rdb.HIncrBy(ctx, codeKey, "attempts", 1).Result()
	if err != nil {
		return err
	}
	if int(attempts) > s.maxAttempts {
		s.rdb.Del(ctx, codeKey)
		return ErrResetTooManyAttempts
	}

	if !hmac.Equal([]byte(stored), []byte(s.hashCode(key, strings.TrimSpace(code)))) {
		return ErrResetCodeInvalid
	}

	s.rdb.Del(ctx, codeKey)
	return nil
}

// Clear 在密碼真的換成功之後把寄送次數歸零，免得使用者剛重設完又立刻被擋。
func (s *ResetService) Clear(ctx context.Context, email string) {
	if s.rdb == nil {
		return
	}
	s.rdb.Del(ctx, "pwreset:sends:"+emailKey(email))
}

func (s *ResetService) hashCode(emailKey, code string) string {
	mac := hmac.New(sha256.New, s.secret)
	fmt.Fprintf(mac, "%s:%s", emailKey, code)
	return hex.EncodeToString(mac.Sum(nil))
}

// 大小寫不同的 email 是同一個帳號（GetUserByEmail 也是 lower 比對），
// key 不統一的話換個大小寫就能繞過次數限制。
func emailKey(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// randomCode 回傳 6 位數字，不足位補 0。用 rand.Int 取範圍而不是取餘數，
// 後者會讓小的數字出現機率略高。
func randomCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}
