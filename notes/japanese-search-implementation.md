# 日本語検索実装の工夫とノウハウ

## 概要

このドキュメントでは、iEPG番組表検索システムにおける日本語検索の実装技術と工夫について詳述します。日本語特有の課題（文字種の多様性、放送業界固有の文字、エンコーディング互換性）に対する包括的なソリューションを提供しています。

## 1. テキスト正規化システム

### 1.1 Unicode正規化 (NFKC)
**実装場所**: `models/normalizer.go:NormalizeText()`

```go
// 互換等価文字を正規等価文字に変換
text = norm.NFKC.String(text)
```

**効果**:
- 囲み文字（①→1）、組文字（㍻→平成）の統一
- 異体字の基本文字への統一
- 検索時の表記ゆれ解決

### 1.2 全角→半角変換
```go
// アルファベット・数字・記号の半角化
text = width.Narrow.String(text)
```

**対象文字**:
- 全角英数字 → 半角英数字
- 全角記号 → 半角記号（一部）
- 全角スペース → 半角スペース

### 1.3 大文字→小文字変換
```go
text = strings.ToLower(text)
```

**検索精度向上**: 大文字小文字の違いを無視した検索を実現

### 1.4 空白・改行正規化
```go
// 改行を半角スペースに変換
text = strings.ReplaceAll(text, "\n", " ")
// 連続する空白を単一スペースに統一
re := regexp.MustCompile(`\s+`)
text = re.ReplaceAllString(text, " ")
```

## 2. ARIB外字対応

### 2.1 放送業界特殊文字
**実装場所**: `models/aribgaiji.go`

**対応文字一覧**:
```go
var aribGaijiMap = map[rune]string{
    0x7A50: "[HV]",     // ハイビジョン
    0x7A51: "[SD]",     // 標準画質
    0x7A52: "[字]",     // 字幕
    0x7A53: "[双]",     // 二ヶ国語
    0x7A54: "[再]",     // 再放送
    0x7A55: "[新]",     // 新番組
    // ... 32種類の放送用記号
}
```

### 2.2 Unicode絵文字マッピング
```go
var unicodeEmojiMap = map[rune]string{
    0x1F21A: "[無]",    // 🈚 無料
    0x1F22F: "[指]",    // 🈯 指定席
    0x26C5:  "[曇]",    // ⛅ 曇り
    // ... 天気記号等の対応
}
```

**用途**:
- 番組表での視覚的表現の統一
- 検索時の文字一致精度向上
- レガシーシステムとの互換性確保

## 3. 検索アルゴリズム

### 3.1 AND検索（基本動作）
**実装場所**: `db/database.go:SearchPrograms()`

```go
// スペース区切りの全キーワードを必須とする
keywords := strings.Fields(normalizedQuery)
for _, keyword := range keywords {
    if !strings.HasPrefix(keyword, "-") && !isQuoted(keyword) {
        conditions = append(conditions, 
            "(p.nameForSearch LIKE ? OR p.descForSearch LIKE ?)")
        args = append(args, "%"+keyword+"%", "%"+keyword+"%")
    }
}
```

**特徴**:
- 2025年にOR検索からAND検索に変更
- より絞り込まれた精度の高い検索結果
- 複数キーワードでの詳細検索が可能

### 3.2 フレーズ検索
```go
// ダブルクォーテーション囲みの語順保持検索
if strings.HasPrefix(keyword, "\"") && strings.HasSuffix(keyword, "\"") {
    phrase := strings.Trim(keyword, "\"")
    conditions = append(conditions, 
        "(p.nameForSearch LIKE ? OR p.descForSearch LIKE ?)")
    args = append(args, "%"+phrase+"%", "%"+phrase+"%")
}
```

**用途**:
- 番組タイトルの正確な検索
- 特定フレーズの語順を重視した検索

### 3.3 否定検索
```go
// マイナスプレフィックスで除外検索
if strings.HasPrefix(keyword, "-") {
    excludeKeyword := strings.TrimPrefix(keyword, "-")
    conditions = append(conditions, 
        "(p.nameForSearch NOT LIKE ? AND p.descForSearch NOT LIKE ?)")
    args = append(args, "%"+excludeKeyword+"%", "%"+excludeKeyword+"%")
}
```

**活用例**:
- `-ニュース`: ニュース番組を除外
- `-再放送`: 再放送を除外した新作番組の検索

### 3.4 複合検索
```
検索例: "今日のニュース" 政治 -スポーツ
→ 「今日のニュース」を含み、かつ「政治」を含み、かつ「スポーツ」を含まない番組
```

## 4. データベース設計

### 4.1 検索最適化スキーマ
```sql
CREATE TABLE programs (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,           -- 表示用番組名
    nameForSearch TEXT,           -- 検索用正規化済み番組名
    description TEXT,             -- 表示用番組説明
    descForSearch TEXT,           -- 検索用正規化済み番組説明
    -- その他のフィールド
);
```

**設計思想**:
- 表示用と検索用のフィールド分離
- 検索処理の高速化（事前正規化）
- データ整合性の確保

### 4.2 インデックス戦略
```sql
CREATE INDEX idx_programs_search ON programs(nameForSearch, descForSearch);
CREATE INDEX idx_programs_time ON programs(startAt, endAt);
CREATE INDEX idx_programs_channel ON programs(serviceId);
```

## 5. エンコーディング処理

### 5.1 iEPG出力のShift-JIS変換
**実装場所**: `handlers/iepg.go:convertToShiftJIS()`

```go
func convertToShiftJIS(text string) string {
    encoder := japanese.ShiftJIS.NewEncoder()
    encoded, err := encoder.String(text)
    if err != nil {
        // エンコードできない文字の代替処理
        return sanitizeForShiftJIS(text)
    }
    return encoded
}
```

### 5.2 文字サニタイズ処理
```go
func sanitizeForShiftJIS(text string) string {
    result := make([]rune, 0, len(text))
    for _, r := range text {
        switch {
        case isUnicodeEmoji(r):
            result = append(result, []rune("[絵文字]")...)
        case isKanjiOutOfJIS(r):
            result = append(result, []rune("[漢字]")...)
        case isSpecialKana(r):
            result = append(result, []rune("[仮名]")...)
        default:
            if !canEncodeToShiftJIS(r) {
                result = append(result, '・')
            } else {
                result = append(result, r)
            }
        }
    }
    return string(result)
}
```

## 6. フロントエンド検索UI

### 6.1 検索構文ガイダンス
**実装場所**: `static/search.html`

```html
<div class="search-help">
    <h4>検索のコツ</h4>
    <ul>
        <li><code>ニュース スポーツ</code> - すべてを含む</li>
        <li><code>"今日のニュース"</code> - 語順通りに含む</li>
        <li><code>-スポーツ</code> - 含まないものを表示</li>
        <li><code>"特集番組" 野球 -ニュース</code> - 組み合わせ</li>
    </ul>
</div>
```

### 6.2 日本語対応カレンダー
```javascript
// flatpickrによる日本語ローカライゼーション
flatpickr("#dateRange", {
    locale: "ja",
    mode: "range",
    dateFormat: "Y-m-d",
    defaultDate: [today, tomorrow]
});
```

## 7. パフォーマンス最適化

### 7.1 事前正規化戦略
- **データ挿入時**: 番組情報取得時に検索用フィールドを正規化
- **メリット**: 検索時の正規化処理コストを削減
- **トレードオフ**: ストレージ使用量の微増

### 7.2 除外チャンネル処理
```go
// 検索時に除外設定チャンネルを自動フィルタリング
excludedServices := getExcludedServices()
if len(excludedServices) > 0 {
    placeholders := strings.Repeat("?,", len(excludedServices)-1) + "?"
    conditions = append(conditions, "p.serviceId NOT IN ("+placeholders+")")
}
```

## 8. テスト戦略

### 8.1 正規化テスト
**実装場所**: `models/normalizer_test.go` (想定)

```go
func TestNormalizeText(t *testing.T) {
    testCases := []struct {
        input    string
        expected string
    }{
        {"ニュース　７", "ニュース 7"},              // 全角→半角
        {"①②③", "123"},                        // 囲み文字
        {"㍻㍿", "平成株式会社"},                    // 組文字
        {"\"特集番組\"", "\"特集番組\""},            // クォート保持
    }
    
    for _, tc := range testCases {
        result := NormalizeText(tc.input)
        assert.Equal(t, tc.expected, result)
    }
}
```

### 8.2 検索機能テスト
```go
func TestSearchPrograms(t *testing.T) {
    // AND検索テスト
    results := SearchPrograms("ニュース スポーツ")
    // 結果に両方のキーワードが含まれることを確認
    
    // フレーズ検索テスト  
    results = SearchPrograms("\"今日のニュース\"")
    // 語順が保持されることを確認
    
    // 否定検索テスト
    results = SearchPrograms("-再放送")
    // 再放送が除外されることを確認
}
```

## 9. 運用上の考慮事項

### 9.1 文字コード互換性
- **UTF-8**: 内部処理とAPI通信
- **Shift-JIS**: iEPG出力（レガシー互換性）
- **エラーハンドリング**: エンコードできない文字の適切な代替

### 9.2 放送業界標準への準拠
- **ARIB規格**: 放送用外字の完全サポート
- **番組表規格**: EPG（電子番組ガイド）標準との互換性
- **更新対応**: 新しい放送用記号の追加に対する拡張性

### 9.3 パフォーマンス監視
- **検索レスポンス時間**: 通常100ms以下を目標
- **データベース負荷**: インデックス効率の定期確認
- **メモリ使用量**: 正規化処理のメモリ効率監視

## 10. 今後の改善案

### 10.1 形態素解析の導入
- **MeCab/Kuromoji**: より精密な日本語解析
- **読み検索**: ひらがな・カタカナでの番組名検索
- **類義語検索**: シノニム辞書による検索拡張

### 10.2 検索精度向上
- **TF-IDF**: 文書頻度による関連度スコアリング
- **ファジー検索**: 誤字脱字に対する寛容な検索
- **学習機能**: ユーザー検索履歴からの改善

### 10.3 多言語対応
- **英語番組**: 海外番組の検索対応
- **韓国語・中国語**: アジア圏番組への対応拡張

## まとめ

このシステムの日本語検索実装は、以下の特徴により高精度で実用的な検索体験を提供しています：

1. **包括的な文字正規化**: Unicode標準に準拠した確実な文字統一
2. **放送業界特化**: ARIB外字など業界固有要求への完全対応  
3. **柔軟な検索構文**: AND/フレーズ/否定検索の組み合わせ
4. **エンコーディング互換性**: モダンなUTF-8とレガシーShift-JISの両立
5. **パフォーマンス最適化**: 事前正規化による高速検索の実現

これらの技術的工夫により、日本語番組表検索システムとして高い実用性と信頼性を確保しています。