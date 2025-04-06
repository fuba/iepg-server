# IEPG Server

IEPGフォーマットでテレビ番組情報を提供するサーバーです。Mirakurunと連携して動作します。

## 機能

- Mirakurunからの番組情報の取得と保存
- 番組検索機能（キーワード、チャンネル、時間範囲による検索）
- IEPG形式での番組詳細情報の提供
- Webベースの検索UI

## インストール方法

### 必要な環境

- Docker および Docker Compose
- Mirakurun（[Mirakurun](https://github.com/Chinachu/Mirakurun)）
- 十分なディスク容量（番組情報保存用）

### 手順

1. リポジトリをクローン

```bash
git clone https://github.com/yourusername/iepg-server.git
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

4. ビルドと起動

```bash
docker-compose up -d
```

5. 動作確認

ブラウザで `http://localhost:40870/ui/search` にアクセスして検索UIが表示されることを確認します。

## バージョンアップ方法

1. 最新のコードを取得

```bash
git pull
```

2. 既存のコンテナを停止して削除

```bash
docker-compose down
```

3. 新しいイメージをビルドして再起動

```bash
docker-compose build --no-cache
docker-compose up -d
```

4. ログを確認

```bash
docker-compose logs -f
```

## API仕様

### 番組検索 API

**エンドポイント**: `/search`  
**メソッド**: GET  
**説明**: 指定された条件に一致する番組を検索します。

**クエリパラメータ**:
- `q` (オプション): 検索キーワード（番組名や説明文に含まれるテキスト）
- `serviceId` (オプション): サービスID（チャンネルのID）
- `startFrom` (オプション): 開始時間の下限（UNIXタイムスタンプ、ミリ秒）
- `startTo` (オプション): 開始時間の上限（UNIXタイムスタンプ、ミリ秒）

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

**エンドポイント**: `/program/{id}`  
**メソッド**: GET  
**説明**: 指定されたIDの番組情報をIEPG形式で取得します。

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

## ライセンス

[MIT License](LICENSE)