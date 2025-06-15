FROM golang:1.23-alpine AS builder

WORKDIR /build

# 必要なパッケージのインストール
RUN apk add --no-cache gcc musl-dev

# ソースコードのコピーと依存関係の設定
COPY . .
RUN go mod tidy
RUN go mod download

# ビルド
RUN CGO_ENABLED=1 GOOS=linux go build -a -o iepg-server .

# 最終イメージ
FROM alpine:latest

WORKDIR /app

# 必要なランタイム依存をインストール
RUN apk add --no-cache ca-certificates tzdata sqlite wget

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

# ヘルスチェック
HEALTHCHECK --interval=30s --timeout=10s --start-period=40s --retries=3 \
  CMD wget --quiet --tries=1 --spider http://localhost:40870/services || exit 1

# 実行
CMD ["./iepg-server"]