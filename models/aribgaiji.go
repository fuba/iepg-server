// models/aribgaiji.go
//
// このファイルは以下のリポジトリのaribgaiji.pyを翻案して作成しました:
// https://github.com/murakamiy/epgdump_py/blob/master/aribgaiji.py
//
// Original Copyright (C) 2011 Yasumasa Murakami. All Rights Reserved.
// Licensed under the MIT License, which is reproduced below:
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package models

// ARIBの外字マップ（タイトル用）
var ARIBGaijiMapTitle = map[rune]string{
	0x7A50: "[HV]",
	0x7A51: "[SD]",
	0x7A52: "[Ｐ]",
	0x7A53: "[Ｗ]",
	0x7A54: "[MV]",
	0x7A55: "[手]",
	0x7A56: "[字]",
	0x7A57: "[双]",
	0x7A58: "[デ]",
	0x7A59: "[Ｓ]",
	0x7A5A: "[二]",
	0x7A5B: "[多]",
	0x7A5C: "[解]",
	0x7A5D: "[SS]",
	0x7A5E: "[Ｂ]",
	0x7A5F: "[Ｎ]",
	0x7A62: "[天]",
	0x7A63: "[交]",
	0x7A64: "[映]",
	0x7A65: "[無]",
	0x7A66: "[料]",
	0x7A67: "[年齢制限]",
	0x7A68: "[前]",
	0x7A69: "[後]",
	0x7A6A: "[再]",
	0x7A6B: "[新]",
	0x7A6C: "[初]",
	0x7A6D: "[終]",
	0x7A6E: "[生]",
	0x7A6F: "[販]",
	0x7A70: "[声]",
	0x7A71: "[吹]",
	0x7A72: "[PPV]",
}

// ARIBの外字マップ（その他）
// 必要に応じて使用する部分のみを追加
var ARIBGaijiMapSymbols = map[rune]string{
	0x7A60: "■",
	0x7A61: "●",
	0x7A73: "（秘）",
	0x7A74: "ほか",
	0x7C21: "→",
	0x7C22: "←",
	0x7C23: "↑",
	0x7C24: "↓",
	0x7C25: "●",
	0x7C26: "○",
	0x7C3F: "[新]",
	0x7C7A: "[演]",
}

// Unicodeの絵文字・囲み文字マッピング
var UnicodeEmojiMap = map[rune]string{
	// 囲みアルファベット・数字
	0x2460: "①", // ①
	0x2461: "②", // ②
	0x2462: "③", // ③
	0x2463: "④", // ④
	0x2464: "⑤", // ⑤
	0x2465: "⑥", // ⑥
	0x2466: "⑦", // ⑦
	0x2467: "⑧", // ⑧
	0x2468: "⑨", // ⑨
	0x2469: "⑩", // ⑩
	
	// 囲み漢字 (Enclosed Ideographic Supplement 0x1F200-0x1F2FF)
	0x1F200: "[ほか]", // 🈀 (SQUARED HIRAGANA HOKA)
	0x1F201: "[ココ]", // 🈁 (SQUARED KATAKANA KOKO)
	0x1F202: "[サ]",  // 🈂 (SQUARED KATAKANA SA)
	0x1F210: "[手]",  // 🈐 (SQUARED CJK UNIFIED IDEOGRAPH-624B)
	0x1F211: "[字]",  // 🈑 (SQUARED CJK UNIFIED IDEOGRAPH-5B57)
	0x1F212: "[双]",  // 🈒 (SQUARED CJK UNIFIED IDEOGRAPH-53CC)
	0x1F213: "[デ]",  // 🈓 (SQUARED CJK UNIFIED IDEOGRAPH-30C7)
	0x1F214: "[二]",  // 🈔 (SQUARED CJK UNIFIED IDEOGRAPH-4E8C)
	0x1F215: "[多]",  // 🈕 (SQUARED CJK UNIFIED IDEOGRAPH-591A)
	0x1F216: "[解]",  // 🈖 (SQUARED CJK UNIFIED IDEOGRAPH-89E3)
	0x1F217: "[天]",  // 🈗 (SQUARED CJK UNIFIED IDEOGRAPH-5929)
	0x1F218: "[交]",  // 🈘 (SQUARED CJK UNIFIED IDEOGRAPH-4EA4)
	0x1F219: "[映]",  // 🈙 (SQUARED CJK UNIFIED IDEOGRAPH-6620)
	0x1F21A: "[無]",  // 🈚 (SQUARED CJK UNIFIED IDEOGRAPH-7121)
	0x1F21B: "[料]",  // 🈛 (SQUARED CJK UNIFIED IDEOGRAPH-6599)
	0x1F21C: "[前]",  // 🈜 (SQUARED CJK UNIFIED IDEOGRAPH-524D)
	0x1F21D: "[後]",  // 🈝 (SQUARED CJK UNIFIED IDEOGRAPH-5F8C)
	0x1F21E: "[再]",  // 🈞 (SQUARED CJK UNIFIED IDEOGRAPH-518D)
	0x1F21F: "[新]",  // 🈟 (SQUARED CJK UNIFIED IDEOGRAPH-65B0)
	0x1F220: "[初]",  // 🈠 (SQUARED CJK UNIFIED IDEOGRAPH-521D)
	0x1F221: "[終]",  // 🈡 (SQUARED CJK UNIFIED IDEOGRAPH-7D42)
	0x1F222: "[生]",  // 🈢 (SQUARED CJK UNIFIED IDEOGRAPH-751F)
	0x1F223: "[販]",  // 🈣 (SQUARED CJK UNIFIED IDEOGRAPH-8CA9)
	0x1F224: "[声]",  // 🈤 (SQUARED CJK UNIFIED IDEOGRAPH-58F0)
	0x1F225: "[吹]",  // 🈥 (SQUARED CJK UNIFIED IDEOGRAPH-5439)
	0x1F226: "[演]",  // 🈦 (SQUARED CJK UNIFIED IDEOGRAPH-6F14)
	0x1F227: "[投]",  // 🈧 (SQUARED CJK UNIFIED IDEOGRAPH-6295)
	0x1F228: "[捕]",  // 🈨 (SQUARED CJK UNIFIED IDEOGRAPH-6355)
	0x1F229: "[一]",  // 🈩 (SQUARED CJK UNIFIED IDEOGRAPH-4E00)
	0x1F22A: "[二]",  // 🈪 (SQUARED CJK UNIFIED IDEOGRAPH-4E8C)
	0x1F22B: "[三]",  // 🈫 (SQUARED CJK UNIFIED IDEOGRAPH-4E09)
	0x1F22C: "[四]",  // 🈬 (SQUARED CJK UNIFIED IDEOGRAPH-56DB)
	0x1F22D: "[五]",  // 🈭 (SQUARED CJK UNIFIED IDEOGRAPH-4E94)
	0x1F22E: "[六]",  // 🈮 (SQUARED CJK UNIFIED IDEOGRAPH-516D)
	0x1F22F: "[指]",  // 🈯 (SQUARED CJK UNIFIED IDEOGRAPH-6307)
	0x1F230: "[七]",  // 🈰 (SQUARED CJK UNIFIED IDEOGRAPH-4E03)
	0x1F231: "[八]",  // 🈱 (SQUARED CJK UNIFIED IDEOGRAPH-516B)
	0x1F232: "[禁]",  // 🈲 (SQUARED CJK UNIFIED IDEOGRAPH-7981)
	0x1F233: "[空]",  // 🈳 (SQUARED CJK UNIFIED IDEOGRAPH-7A7A)
	0x1F234: "[合]",  // 🈴 (SQUARED CJK UNIFIED IDEOGRAPH-5408)
	0x1F235: "[満]",  // 🈵 (SQUARED CJK UNIFIED IDEOGRAPH-6E80)
	0x1F236: "[有]",  // 🈶 (SQUARED CJK UNIFIED IDEOGRAPH-6709)
	0x1F237: "[月]",  // 🈷 (SQUARED CJK UNIFIED IDEOGRAPH-6708)
	0x1F238: "[申]",  // 🈸 (SQUARED CJK UNIFIED IDEOGRAPH-7533)
	0x1F239: "[割]",  // 🈹 (SQUARED CJK UNIFIED IDEOGRAPH-5272)
	0x1F23A: "[営]",  // 🈺 (SQUARED CJK UNIFIED IDEOGRAPH-55B6)
	0x1F23B: "[祝]",  // 🈻 (SQUARED CJK UNIFIED IDEOGRAPH-795D)
	0x1F240: "[富]",  // 🉀 (SQUARED CJK UNIFIED IDEOGRAPH-5BCC)
	0x1F241: "[祝日]",  // 🉁 (SQUARED CJK UNIFIED IDEOGRAPH-795D)
	0x1F242: "[株]",  // 🉂 (SQUARED CJK UNIFIED IDEOGRAPH-682A)
	0x1F243: "[社]",  // 🉃 (SQUARED CJK UNIFIED IDEOGRAPH-793E)
	0x1F244: "[名]",  // 🉄 (SQUARED CJK UNIFIED IDEOGRAPH-540D)
	0x1F245: "[特]",  // 🉅 (SQUARED CJK UNIFIED IDEOGRAPH-7279)
	0x1F246: "[財]",  // 🉆 (SQUARED CJK UNIFIED IDEOGRAPH-8CA1)
	0x1F247: "[祭]",  // 🉇 (SQUARED CJK UNIFIED IDEOGRAPH-796D)
	0x1F248: "[労]",  // 🉈 (SQUARED CJK UNIFIED IDEOGRAPH-5B66)
	0x1F250: "[得]",  // 🉐 (CIRCLED IDEOGRAPH ADVANTAGE)
	0x1F251: "[可]",  // 🉑 (CIRCLED IDEOGRAPH ACCEPT)
	
	// 天気関連
	0x2600: "☀", // ☀ 晴れ
	0x2601: "☁", // ☁ 曇り
	0x2602: "☂", // ☂ 雨
	0x2603: "☃", // ☃ 雪
	0x26A1: "⚡", // ⚡ 雷
}

// ARIBGaijiMapAll は全ての外字マップを統合したマップ
var ARIBGaijiMapAll map[rune]string

// init 関数で統合マップを初期化する
func init() {
	ARIBGaijiMapAll = make(map[rune]string)
	
	// タイトル用外字マップを追加
	for k, v := range ARIBGaijiMapTitle {
		ARIBGaijiMapAll[k] = v
	}
	
	// シンボル用外字マップを追加
	for k, v := range ARIBGaijiMapSymbols {
		ARIBGaijiMapAll[k] = v
	}
	
	// Unicode絵文字マップを追加
	for k, v := range UnicodeEmojiMap {
		ARIBGaijiMapAll[k] = v
	}
}