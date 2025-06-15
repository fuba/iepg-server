# Docker Compose で iEPG Server を利用する

## クイックスタート

```bash
# リポジトリをクローン
git clone <repository-url>
cd iepg-server

# 起動（自動ビルド）
docker compose up --build

# バックグラウンドで起動
docker compose up -d --build
```

## 基本操作

```bash
# サービス起動
docker compose up --build

# バックグラウンドで起動
docker compose up -d --build

# イメージビルドのみ
docker compose build

# サービス停止
docker compose down

# サービス再起動
docker compose restart

# ログ表示
docker compose logs -f

# コンテナ内でシェル実行
docker compose exec iepg-server sh

# Docker リソース削除
docker compose down -v
docker system prune -f
```

## アクセス方法

サービス起動後、以下のURLでアクセスできます：

- **Web UI**: http://localhost:40870
- **番組検索**: http://localhost:40870/ui/search
- **自動予約管理**: http://localhost:40870/ui/auto-reservation
- **チャンネル除外設定**: http://localhost:40870/ui/exclude-channels

## 設定

### 環境変数

`docker-compose.yml` で以下の環境変数を設定できます：

```yaml
environment:
  - PORT=40870                          # サーバーポート
  - LOG_LEVEL=info                      # ログレベル (debug/info/warn/error)
  - DB_PATH=/app/data/programs.db       # データベースファイルパス
  - MIRAKURUN_URL=http://localhost:40772/api  # MirakurunのURL
  - RECORDER_URL=http://localhost:37569       # 録画サーバーのURL
  - ENABLE_AUTO_RESERVATION=true        # 自動予約エンジンの有効化
  - ENABLE_CLEANUP=true                 # 古い番組データの自動削除
  - SKIP_INITIAL_LOAD=false             # 起動時の番組データ初期ロードをスキップ
```

### ボリューム

- `./data:/app/data` - データベースファイルの永続化

### ネットワーク

`network_mode: "host"` を使用してホストネットワークで動作します。
これによりMirakurunや録画サーバーにアクセスできます。

## 開発モード

開発時は以下のコマンドで起動すると便利です：

```bash
docker compose -f docker-compose.yml -f docker-compose.override.yml up --build
```

開発モードでは：
- ログレベルがdebugに設定
- 静的ファイルがマウントされ、変更が即座に反映
- 初期番組データロードがスキップされる

## ヘルスチェック

コンテナには自動ヘルスチェックが設定されています：

```bash
# ヘルスチェック状態確認
docker compose ps

# 手動ヘルスチェック
curl http://localhost:40870/services
```

## トラブルシューティング

### コンテナが起動しない場合

```bash
# ログを確認
docker compose logs iepg-server

# コンテナ内でシェル実行
docker compose exec iepg-server sh
```

### データベースのリセット

```bash
# サービス停止
docker compose down

# データディレクトリ削除
rm -rf ./data

# 再起動（新しいDBが作成される）
docker compose up --build
```

### ポート競合の解決

ポート40870が使用中の場合、`docker-compose.yml`を編集：

```yaml
environment:
  - PORT=8080  # 別のポートに変更
```

## 自動予約機能

### キーワードベース自動予約

1. Web UI (http://localhost:40870) にアクセス
2. 番組検索で候補を検索
3. 「自動予約」ボタンをクリック
4. キーワードを調整して保存

### 管理画面

自動予約管理画面 (http://localhost:40870/ui/auto-reservation) で：
- ルールの作成・編集・削除
- 実行ログの確認
- ルールの有効/無効切り替え

### API使用例

```bash
# 自動予約ルール一覧
curl http://localhost:40870/auto-reservations/rules

# 新しいキーワードルール作成
curl -X POST http://localhost:40870/auto-reservations/rules \
  -H "Content-Type: application/json" \
  -d '{
    "type": "keyword",
    "name": "アニメ自動予約",
    "enabled": true,
    "priority": 10,
    "recorderUrl": "http://localhost:37569",
    "keywordRule": {
      "keywords": ["アニメ"],
      "excludeWords": ["再放送", "総集編"]
    }
  }'
```

## メンテナンス

### 定期メンテナンス

システムは以下を自動実行します：
- 古い番組データの削除（24時間以上経過したもの）
- 自動予約ルールの5分間隔実行
- Mirakurunからの番組データ同期

### バックアップ

```bash
# データベースファイルをバックアップ
cp ./data/programs.db ./backup/programs_$(date +%Y%m%d).db
```

### アップデート

```bash
# 最新コードをプル
git pull

# 再ビルドして起動
docker compose up --build
```