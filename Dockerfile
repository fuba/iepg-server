FROM golang:1.21-alpine AS builder

WORKDIR /build

# 必要なパッケージのインストール
RUN apk add --no-cache gcc musl-dev

# 依存関係コピー＆ダウンロード
COPY go.mod go.sum ./
RUN go mod download

# ソースコードのコピー
COPY . .

# ビルド
RUN CGO_ENABLED=1 GOOS=linux go build -a -o iepg-server .

# 最終イメージ
FROM alpine:latest

WORKDIR /app

# 必要なランタイム依存をインストール
RUN apk add --no-cache ca-certificates tzdata sqlite

# タイムゾーンを設定
ENV TZ=Asia/Tokyo

# アプリケーションバイナリをコピー
COPY --from=builder /build/iepg-server .

# 静的ファイルをコピー
COPY --from=builder /build/static/ ./static/

# データディレクトリを作成
RUN mkdir -p /app/data

# ポート公開
EXPOSE 40870

# 実行
CMD ["./iepg-server"]