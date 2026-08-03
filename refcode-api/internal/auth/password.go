package auth

import (
	"errors"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

var ErrWeakPassword = errors.New("密碼至少 8 個字元")

func HashPassword(plain string) (string, error) {
	// bcrypt 只吃前 72 bytes，超過的部分會被無聲截斷。
	if len(plain) > 72 {
		return "", errors.New("密碼過長（上限 72 bytes）")
	}
	if utf8.RuneCountInString(plain) < 8 {
		return "", ErrWeakPassword
	}
	h, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	return string(h), err
}

func VerifyPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}
