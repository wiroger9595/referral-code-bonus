package auth

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/coreos/go-oidc/v3/oidc"
)

const (
	ProviderGoogle = "google"
	ProviderApple  = "apple"
)

var ErrUnsupportedProvider = errors.New("不支援的登入方式")

type OIDCIdentity struct {
	Provider      string
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
	AvatarURL     string
}

type OIDCVerifier struct {
	issuers   map[string]string
	audiences map[string][]string

	mu        sync.Mutex
	verifiers map[string]*oidc.IDTokenVerifier
}

func NewOIDCVerifier(googleClientIDs, appleClientIDs []string) *OIDCVerifier {
	return &OIDCVerifier{
		issuers: map[string]string{
			ProviderGoogle: "https://accounts.google.com",
			ProviderApple:  "https://appleid.apple.com",
		},
		audiences: map[string][]string{
			ProviderGoogle: googleClientIDs,
			ProviderApple:  appleClientIDs,
		},
		verifiers: map[string]*oidc.IDTokenVerifier{},
	}
}

// Verify 驗 client 傳來的 ID token。aud 自己比對而不交給 go-oidc，
// 因為同一個 provider 會有多個合法 client id
// （Apple: app 是 bundle id、web 是 Services ID）。
func (v *OIDCVerifier) Verify(ctx context.Context, provider, rawIDToken string) (*OIDCIdentity, error) {
	auds, ok := v.audiences[provider]
	if !ok {
		return nil, ErrUnsupportedProvider
	}
	if len(auds) == 0 {
		return nil, fmt.Errorf("%s 登入未設定 client id", provider)
	}

	verifier, err := v.verifierFor(ctx, provider)
	if err != nil {
		return nil, err
	}

	idToken, err := verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("ID token 驗證失敗: %w", err)
	}

	matched := false
	for _, aud := range idToken.Audience {
		if slices.Contains(auds, aud) {
			matched = true
			break
		}
	}
	if !matched {
		return nil, errors.New("ID token 的 audience 不在允許清單內")
	}

	var claims struct {
		Email         string `json:"email"`
		EmailVerified any    `json:"email_verified"` // Apple 會給字串 "true"，Google 給 bool
		Name          string `json:"name"`
		Picture       string `json:"picture"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return nil, err
	}

	return &OIDCIdentity{
		Provider:      provider,
		Subject:       idToken.Subject,
		Email:         claims.Email,
		EmailVerified: parseBoolClaim(claims.EmailVerified),
		Name:          claims.Name,
		AvatarURL:     claims.Picture,
	}, nil
}

func (v *OIDCVerifier) verifierFor(ctx context.Context, provider string) (*oidc.IDTokenVerifier, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if ver, ok := v.verifiers[provider]; ok {
		return ver, nil
	}

	// discovery 需要打外網，所以延後到第一次真的有人用這個 provider 登入時才做。
	p, err := oidc.NewProvider(ctx, v.issuers[provider])
	if err != nil {
		return nil, fmt.Errorf("%s discovery 失敗: %w", provider, err)
	}
	ver := p.Verifier(&oidc.Config{SkipClientIDCheck: true})
	v.verifiers[provider] = ver
	return ver, nil
}

func parseBoolClaim(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return t == "true"
	}
	return false
}
