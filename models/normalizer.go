// models/normalizer.go
package models

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
	"golang.org/x/text/width"
)

// NormalizeForSearch は検索用に文字列を正規化する
// - 全角→半角変換
// - 大文字→小文字変換
// - アクセント除去
// - 異体字正規化
func NormalizeForSearch(s string) string {
	if s == "" {
		return ""
	}

	// Unicode正規化 (NFKCで互換等価文字を正規等価文字に変換)
	// これにより、囲み文字、組文字、異体字などが基本文字に分解される
	s = norm.NFKC.String(s)

	// 全角→半角変換 (アルファベット、数字、記号など)
	s = width.Narrow.String(s)

	// 大文字→小文字変換
	var result strings.Builder
	result.Grow(len(s))

	for _, r := range s {
		// 大文字→小文字変換
		r = unicode.ToLower(r)
		result.WriteRune(r)
	}

	// 改行を半角スペースに置き換え
	normalized := result.String()
	normalized = strings.ReplaceAll(normalized, "\n", " ")
	normalized = strings.ReplaceAll(normalized, "\r", " ")
	
	// 連続する空白を単一の空白に置き換え
	for strings.Contains(normalized, "  ") {
		normalized = strings.ReplaceAll(normalized, "  ", " ")
	}

	return normalized
}