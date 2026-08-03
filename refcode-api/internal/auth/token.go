package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"refcode-api/internal/store"
	"refcode-api/internal/store/dbgen"
)

var (
	ErrInvalidToken = errors.New("token 無效")
	ErrTokenReused  = errors.New("refresh token 重用，該裝置已全部撤銷")
)

// Subject 分開 user 與 admin，避免一般使用者的 token 拿去打 admin API。
type Subject string

const (
	SubjectUser  Subject = "user"
	SubjectAdmin Subject = "admin"
)

type Claims struct {
	jwt.RegisteredClaims
	Subject Subject `json:"sub_type"`
	Role    string  `json:"role,omitempty"`
}

type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type Service struct {
	store      *store.Store
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewService(st *store.Store, secret string, accessTTL, refreshTTL time.Duration) *Service {
	return &Service{store: st, secret: []byte(secret), accessTTL: accessTTL, refreshTTL: refreshTTL}
}

func (s *Service) SignAccessToken(subjectID uuid.UUID, kind Subject, role string) (string, time.Time, error) {
	expiresAt := time.Now().Add(s.accessTTL)
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   subjectID.String(),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ID:        uuid.NewString(),
		},
		Subject: kind,
		Role:    role,
	}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
	return tok, expiresAt, err
}

func (s *Service) ParseAccessToken(raw string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("非預期的簽章演算法: %v", t.Header["alg"])
		}
		return s.secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

// IssuePair 開一條新的 token family（登入時用）。
func (s *Service) IssuePair(ctx context.Context, userID uuid.UUID, userAgent string) (*TokenPair, error) {
	return s.issue(ctx, userID, uuid.New(), userAgent)
}

func (s *Service) issue(ctx context.Context, userID, familyID uuid.UUID, userAgent string) (*TokenPair, error) {
	access, expiresAt, err := s.SignAccessToken(userID, SubjectUser, "")
	if err != nil {
		return nil, err
	}

	refresh, err := randomToken()
	if err != nil {
		return nil, err
	}
	if _, err := s.store.CreateRefreshToken(ctx, dbgen.CreateRefreshTokenParams{
		UserID:    userID,
		TokenHash: hashToken(refresh),
		FamilyID:  familyID,
		ExpiresAt: time.Now().Add(s.refreshTTL),
		UserAgent: userAgent,
	}); err != nil {
		return nil, err
	}

	return &TokenPair{AccessToken: access, RefreshToken: refresh, ExpiresAt: expiresAt}, nil
}

// Refresh 換發並旋轉。舊 token 再次出現代表外洩，整族撤銷後要求重新登入。
func (s *Service) Refresh(ctx context.Context, rawRefresh, userAgent string) (*TokenPair, error) {
	row, err := s.store.GetRefreshTokenByHash(ctx, hashToken(rawRefresh))
	if err != nil {
		return nil, ErrInvalidToken
	}
	if row.RevokedAt != nil {
		return nil, ErrInvalidToken
	}
	if row.RotatedAt != nil {
		// 已經換發過的 token 又被拿來用 —— 不是正常 client 的行為。
		if err := s.store.RevokeTokenFamily(ctx, row.FamilyID); err != nil {
			return nil, err
		}
		return nil, ErrTokenReused
	}
	if time.Now().After(row.ExpiresAt) {
		return nil, ErrInvalidToken
	}

	if err := s.store.MarkRefreshTokenRotated(ctx, row.ID); err != nil {
		return nil, err
	}
	return s.issue(ctx, row.UserID, row.FamilyID, userAgent)
}

func (s *Service) Revoke(ctx context.Context, rawRefresh string) error {
	row, err := s.store.GetRefreshTokenByHash(ctx, hashToken(rawRefresh))
	if err != nil {
		return nil // 登出對不存在的 token 不用報錯
	}
	return s.store.RevokeTokenFamily(ctx, row.FamilyID)
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// DB 只存雜湊：資料庫外洩時 token 不能直接拿來登入。
func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
