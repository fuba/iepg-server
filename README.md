# IEPG Server

IEPGフォーマットでテレビ番組情報を提供するサーバーです。Mirakurunと連携して動作します。
現在 IEPG フォーマットに全然準拠できてなさそうなので気にしない人だけ使ってください。

[![Go Test](https://github.com/fuba/iepg-server/actions/workflows/go-test.yml/badge.svg)](https://github.com/fuba/iepg-server/actions/workflows/go-test.yml)
[![Docker Build and Push](https://github.com/fuba/iepg-server/actions/workflows/docker-push.yml/badge.svg)](https://github.com/fuba/iepg-server/actions/workflows/docker-push.yml)

## 機能

- Mirakurunからの番組情報の取得と保存
- 番組検索機能（キーワード、チャンネル、時間範囲による検索）
- IEPG形式での番組詳細情報の提供
- Webベースの検索UI
- 放送種別（地上波/BS/CS）によるフィルタリング機能
- 検索結果から除外したいチャンネルを設定する機能
- **録画予約機能** - 番組の録画予約とステータス管理
- **自動予約機能** - キーワードやシリーズIDによる自動録画予約
- **Web管理UI** - 自動予約ルール管理とチャンネル除外設定のWebインターフェース

## インストール方法

### 必要な環境

- Docker および Docker Compose
- Mirakurun（[Mirakurun](https://github.com/Chinachu/Mirakurun)）
- 十分なディスク容量（番組情報保存用）

### 手順

1. リポジトリをクローン

```bash
git clone https://github.com/fuba/iepg-server.git
cd iepg-server
```

2. データディレクトリを作成

```bash
mkdir -p data
```

3. Docker Composeの設定を確認

`docker-compose.yml` ファイルを確認して、必要に応じて環境変数を調整します。

- `PORT`: サーバーが使用するポート（デフォルト: 40870）
- `LOG_LEVEL`: ログレベル（debug/info/warn/error）
- `DB_PATH`: データベースファイルのパス
- `MIRAKURUN_URL`: MirakurunのAPI URL
- `RECORDER_URL`: 録画サーバーのURL（デフォルト: http://localhost:37569）
- `ENABLE_AUTO_RESERVATION`: 自動予約機能の有効/無効（デフォルト: true）
- `ENABLE_CLEANUP`: 古い番組データのクリーンアップ機能（デフォルト: true）
- `SKIP_INITIAL_LOAD`: 起動時の初期データロードをスキップ（デフォルト: false）

4. ビルドと起動

```bash
docker compose up -d
```

5. 動作確認

ブラウザで以下のURLにアクセスして各機能が利用できることを確認します：

- 番組検索: `http://localhost:40870/ui/search`
- チャンネル除外設定: `http://localhost:40870/ui/exclude-channels`
- 自動予約管理: `http://localhost:40870/ui/auto-reservation`

## バージョンアップ方法

1. 最新のコードを取得

```bash
git pull
```

2. 既存のコンテナを停止して削除

```bash
docker compose down
```

3. 新しいイメージをビルドして再起動

```bash
docker compose build --no-cache
docker compose up -d
```

4. ログを確認

```bash
docker compose logs -f
```

## 開発者向け情報

### テスト実行

テストを実行するには以下のコマンドを使用します：

```bash
# Go標準テストの実行
go test -v ./...

# 特定パッケージのテスト
go test -v ./db
go test -v ./handlers
go test -v ./services

# Dockerを使用したテスト
docker build -t iepg-server-test -f Dockerfile.test .
docker run --rm iepg-server-test
```

### ローカル開発

ローカルでの開発時は以下のコマンドでサーバーを起動できます：

```bash
# 環境変数の設定（例）
export PORT=40870
export LOG_LEVEL=debug
export DB_PATH=./data/programs.db
export MIRAKURUN_URL=http://localhost:40772/api
export RECORDER_URL=http://localhost:37569

# サーバー起動
go run main.go
```

### CI/CD

このプロジェクトではGitHub Actionsを使用して以下の自動化を実施しています：

- **テスト自動化**: pushやプルリクエスト時に自動的にテストが実行されます
- **Docker自動ビルド**: mainブランチへのpushやタグの作成時に自動的にDockerイメージがビルドされGitHub Container Registryに公開されます

## API仕様

### 番組検索 API

**エンドポイント**: `/search`  
**メソッド**: GET  
**説明**: 指定された条件に一致する番組を検索します。

**クエリパラメータ**:
- `q` (オプション): 検索キーワード（番組名や説明文に含まれるテキスト）
  - 通常の検索: 単語をスペースで区切って指定すると、それらの単語のすべてを含む番組を検索します（AND検索）
  - フレーズ検索: ダブルクォーテーション (`"`) で囲むと、その語順で完全に一致するフレーズを検索します（例: `"今日のニュース"`)
  - 否定検索: 単語の前に `-` をつけると、その単語を含まない番組を検索します（例: `-スポーツ`）
  - 複合検索: 上記の検索方法を組み合わせることができます（例: `"特集番組" 野球 -ニュース`）
- `serviceId` (オプション): サービスID（チャンネルのID）
- `startFrom` (オプション): 開始時間の下限（UNIXタイムスタンプ、ミリ秒）
- `startTo` (オプション): 開始時間の上限（UNIXタイムスタンプ、ミリ秒）
- `channelType` (オプション): 放送種別（"GR": 地上波, "BS": BSデジタル, "CS": CSデジタル）
- `excludedServices` (オプション): 検索結果から除外するサービスIDのリスト（カンマ区切り）

**レスポンス**: 番組情報の配列（JSON形式）

```json
[
  {
    "id": 1234,
    "serviceId": 1024,
    "startAt": 1617579600000,
    "duration": 1800000,
    "name": "サンプル番組",
    "description": "これは番組の説明です",
    "stationId": "0001",
    "stationName": "サンプル放送",
    "channelType": "GR",
    "channelNumber": "27",
    "remoteControlKey": 1
  },
  ...
]
```

### サービス一覧 API

**エンドポイント**: `/services`  
**メソッド**: GET  
**説明**: 利用可能なサービス（チャンネル）の一覧を取得します。

**レスポンス**: サービス情報の配列（JSON形式）

```json
[
  {
    "id": 1,
    "serviceId": 1024,
    "networkId": 32736,
    "name": "サンプル放送",
    "type": 1,
    "logoId": 0,
    "remoteControlKeyId": 1,
    "channelType": "GR",
    "channelNumber": "27"
  },
  ...
]
```

### IEPG API

**エンドポイント**: `/program/{id}.tvpid`
**メソッド**: GET
**説明**: 指定されたIDの番組情報をIEPG形式で取得します。拡張子を付けない
`/program/{id}` も後方互換のため利用可能です。

**パスパラメータ**:
- `id`: 番組ID

**レスポンス**: IEPG形式のテキストデータ

```
Content-type: application/x-tv-program-digital-info; charset=shift_jis
version: 2
station: 0001
station-name: サンプル放送
service-id: 1024
channel: 27
type: GR
year: 2023
month: 04
date: 15
start: 12:00
end: 12:30
program-title: サンプル番組
program-id: 1234

これは番組の説明です。
```

### 録画予約 API

#### 予約作成
**エンドポイント**: `/reservations`  
**メソッド**: POST  
**説明**: 指定された番組の録画予約を作成します。

**リクエストボディ**:
```json
{
  "programId": 1234,
  "recorderUrl": "http://localhost:37569" // オプション
}
```

#### 予約一覧取得
**エンドポイント**: `/reservations`  
**メソッド**: GET  
**説明**: 作成された予約の一覧を取得します。

#### 予約削除
**エンドポイント**: `/reservations/{id}`  
**メソッド**: DELETE  
**説明**: 指定されたIDの予約を削除します。

### 自動予約管理 API

#### 自動予約ルール作成
**エンドポイント**: `/auto-reservations/rules`  
**メソッド**: POST  
**説明**: 新しい自動予約ルールを作成します。

**リクエストボディ**:
```json
{
  "type": "keyword", // "keyword" または "series"
  "name": "ルール名",
  "enabled": true,
  "priority": 10,
  "recorderUrl": "http://localhost:37569",
  "keywords": ["キーワード1", "キーワード2"], // type=keywordの場合
  "excludeWords": ["除外ワード"],
  "serviceIds": [1024, 1025], // チャンネル指定（オプション）
  "seriesId": "12345" // type=seriesの場合
}
```

#### 自動予約ルール一覧取得
**エンドポイント**: `/auto-reservations/rules`  
**メソッド**: GET

#### 自動予約ルール詳細取得
**エンドポイント**: `/auto-reservations/rules/{id}`  
**メソッド**: GET

#### 自動予約ルール更新
**エンドポイント**: `/auto-reservations/rules/{id}`  
**メソッド**: PUT

#### 自動予約ルール削除
**エンドポイント**: `/auto-reservations/rules/{id}`  
**メソッド**: DELETE

#### 自動予約実行ログ取得
**エンドポイント**: `/auto-reservations/logs`  
**メソッド**: GET  
**説明**: 自動予約の実行履歴を取得します。

**クエリパラメータ**:
- `ruleId` (オプション): 特定ルールのログのみ取得
- `limit` (オプション): 取得件数の上限

### チャンネル除外設定 API

#### 除外チャンネル追加
**エンドポイント**: `/services/exclude`  
**メソッド**: POST

#### 除外チャンネル削除
**エンドポイント**: `/services/unexclude`  
**メソッド**: POST

#### 除外チャンネル一覧取得
**エンドポイント**: `/services/excluded`  
**メソッド**: GET

## ライセンス

[MIT License](LICENSE)