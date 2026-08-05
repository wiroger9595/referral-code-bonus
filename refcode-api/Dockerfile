# Northflank 從 repo 根目錄 build，所以路徑都帶 refcode-api/ 前綴。
# 這支只負責跑 api——migration 不在這裡跑，正式資料庫要人工確認過再套用
# （見 README 的 make migrate-up）。

FROM golang:1.26-alpine AS build
WORKDIR /src

# 先只複製依賴清單，讓 go mod download 這層在原始碼變動時還能命中快取
COPY refcode-api/go.mod refcode-api/go.sum ./
RUN go mod download

COPY refcode-api/ ./

# CGO 關掉才能在 scratch/alpine 上跑靜態執行檔；ldflags 去掉除錯資訊縮小體積
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

FROM alpine:3.21
# HTTPS 出站（Supabase、Cloudinary、SMTP）要根憑證，alpine 預設沒有
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 10001 app
USER app

COPY --from=build /out/api /usr/local/bin/api

# 實際的位址由 Northflank 的環境變數 HTTP_ADDR 決定，這裡給一個對得上 EXPOSE 的預設
ENV HTTP_ADDR=:8080
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/api"]
