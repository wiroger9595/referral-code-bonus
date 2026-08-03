// Package kv 是 redis 連線。目前只有忘記密碼的驗證碼與寄送次數在用 ——
// 兩者都是短命、到期就該自己消失的資料，放 Postgres 反而要多養一張表跟一支清理排程。
package kv

import (
	"context"
	"crypto/tls"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type Options struct {
	Addr     string
	Password string
	DB       int
	UseTLS   bool
}

func New(ctx context.Context, opts Options) (*redis.Client, error) {
	o := &redis.Options{
		Addr:     opts.Addr,
		Password: opts.Password,
		DB:       opts.DB,
	}
	if opts.UseTLS {
		// 託管的 redis（Upstash、ElastiCache 之類）走 TLS，本機那台通常沒有。
		o.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}

	client := redis.NewClient(o)
	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("連不上 redis（%s）: %w", opts.Addr, err)
	}
	return client, nil
}
