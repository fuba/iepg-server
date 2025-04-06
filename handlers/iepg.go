// handlers/iepg.go
package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/encoding/japanese"

	"github.com/fuba/iepg-server/db"
	"github.com/fuba/iepg-server/models"
)

// HandleIEPG は /program/{id}.tvpid エンドポイントのハンドラー
func HandleIEPG(w http.ResponseWriter, r *http.Request, dbConn *sql.DB) {
	models.Log.Debug("HandleIEPG: Processing request from %s: %s", r.RemoteAddr, r.URL.Path)
	
	// URL例: /program/123.tvpid
	path := r.URL.Path
	idStr := strings.TrimPrefix(path, "/program/")
	idStr = strings.TrimSuffix(idStr, ".tvpid")
	
	models.Log.Debug("HandleIEPG: Extracted program ID: %s", idStr)
	
	if idStr == "" {
		models.Log.Error("HandleIEPG: Program ID not provided")
		http.Error(w, "program id not provided", http.StatusBadRequest)
		return
	}
	
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		models.Log.Error("HandleIEPG: Invalid program ID: %s, error: %v", idStr, err)
		http.Error(w, "invalid program id", http.StatusBadRequest)
		return
	}
	
	models.Log.Debug("HandleIEPG: Looking up program with ID: %d", id)
	
	p, err := db.GetProgramByID(dbConn, id)
	if err != nil {
		models.Log.Error("HandleIEPG: Program not found, ID: %d, error: %v", id, err)
		http.Error(w, "program not found", http.StatusNotFound)
		return
	}
	
	models.Log.Info("HandleIEPG: Found program: ID=%d, Name=%s", p.ID, p.Name)

	// 開始時刻と終了時刻の算出
	startTime := time.UnixMilli(p.StartAt)
	endTime := startTime.Add(time.Duration(p.Duration) * time.Millisecond)
	
	models.Log.Debug("HandleIEPG: Program times - Start: %s, End: %s", 
		startTime.Format(time.RFC3339), endTime.Format(time.RFC3339))
		
	// 特殊文字を適切な代替表現に置換
	sanitizedName := normalizeSpecialCharacters(p.Name)
	sanitizedDescription := normalizeSpecialCharacters(p.Description)
	models.Log.Debug("HandleIEPG: Original name: %s, Sanitized name: %s", p.Name, sanitizedName)
	models.Log.Debug("HandleIEPG: Original desc: %s, Sanitized desc: %s", p.Description, sanitizedDescription)

	// 番組に対応するサービス（テレビ局）情報を取得
	var stationId, stationName, channelType, channelNumber string
	var serviceId int64
	if service, ok := models.ServiceMapInstance.Get(p.ServiceID); ok {
		// リモコンキーIDあるいはサービスIDを文字列に変換
		if service.RemoteControlKeyID > 0 {
			stationId = fmt.Sprintf("%04d", service.RemoteControlKeyID)
		} else {
			stationId = fmt.Sprintf("%04d", service.ServiceID)
		}
		stationName = service.Name
		serviceId = service.ServiceID
		channelType = service.ChannelType
		channelNumber = service.ChannelNumber
		
		models.Log.Debug("HandleIEPG: Found service: ID=%d, Name=%s, RemoteControlKeyID=%d", 
			service.ServiceID, service.Name, service.RemoteControlKeyID)
	} else {
		// サービス情報が見つからない場合はダミー情報を使用
		stationId = "0000"
		stationName = "未知の放送局"
		serviceId = p.ServiceID
		channelType = "unknown"
		channelNumber = "0"
		models.Log.Debug("HandleIEPG: Service not found for ServiceID=%d, using dummy station info", p.ServiceID)
	}
	
	// 簡易 iEPG 出力
	iepg := "Content-type: application/x-tv-program-digital-info; charset=shift_jis\r\n"
	iepg += "version: 2\r\n"
	iepg += "station: " + stationName + "\r\n"
	iepg += "station-id: " + stationId + "\r\n"
	iepg += "service-id: " + strconv.FormatInt(serviceId, 10) + "\r\n"
	iepg += "channel: " + channelNumber + "\r\n"
	iepg += "type: " + channelType + "\r\n"
	iepg += "year: " + strconv.Itoa(startTime.Year()) + "\r\n"
	iepg += "month: " + padZero(int(startTime.Month())) + "\r\n"
	iepg += "date: " + padZero(startTime.Day()) + "\r\n"
	iepg += "start: " + startTime.Format("15:04") + "\r\n"
	iepg += "end: " + endTime.Format("15:04") + "\r\n"
	iepg += "program-title: " + sanitizedName + "\r\n"
	iepg += "program-id: " + strconv.FormatInt(p.ID, 10) + "\r\n"
	// 番組説明を空行を挟んで出力
	if sanitizedDescription != "" {
		iepg += "\r\n" + sanitizedDescription + "\r\n"
	}
	
	models.Log.Debug("HandleIEPG: Generated iEPG data:\n%s", iepg)

	// すべての文字列をShift-JIS安全な文字に変換
	iepg = sanitizeForShiftJIS(iepg)
	
	// Shift_JISにエンコードして出力
	w.Header().Set("Content-Type", "application/x-tv-program-digital-info; charset=shift_jis")
	encoder := japanese.ShiftJIS.NewEncoder()
	sjisData, err := encoder.String(iepg)
	if err != nil {
		models.Log.Error("HandleIEPG: Shift-JIS encoding error: %v", err)
		http.Error(w, "encoding error", http.StatusInternalServerError)
		return
	}
	
	models.Log.Debug("HandleIEPG: Successfully encoded to Shift-JIS, sending response")
	w.Write([]byte(sjisData))
	models.Log.Debug("HandleIEPG: Response sent successfully")
}

func padZero(n int) string {
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}

// normalizeSpecialCharacters は、特殊文字を適切な代替表現に置換する
// ARIBの外字や様々な符号化文字セットに対応する
func normalizeSpecialCharacters(s string) string {
	var result strings.Builder
	result.Grow(len(s))

	for _, r := range s {
		// ARIB外字コード範囲のチェック (0x7A50-0x7E7D)
		if (r >= 0x7A50 && r <= 0x7A7B) || 
		   (r >= 0x7C21 && r <= 0x7C7B) || 
		   (r >= 0x7D21 && r <= 0x7D7B) || 
		   (r >= 0x7E21 && r <= 0x7E7D) {
			// ARIB外字マップから変換
			if replacement, ok := models.ARIBGaijiMapAll[r]; ok {
				result.WriteString(replacement)
			} else {
				// マップにない場合は「・」で代替
				result.WriteString("・")
			}
		} else if r >= 0x1F000 { 
			// Unicode絵文字範囲 (Emoji, Pictographs, etc.)
			if replacement, ok := models.ARIBGaijiMapAll[r]; ok {
				// マップに登録されている場合はその変換を使用
				result.WriteString(replacement)
			} else if r >= 0x1F200 && r <= 0x1F2FF {
				// Enclosed Ideographic Supplement (特に囲み漢字)
				// 対応する漢字を取得して囲み形式にする
				var baseChar rune
				
				// 代表的な漢字コード (参考情報)
				switch r {
				case 0x1F200: // SQUARED HIRAGANA HOKA
					result.WriteString("[ほか]")
					continue
				case 0x1F201: // SQUARED KATAKANA KOKO
					result.WriteString("[ココ]")
					continue
				case 0x1F21A: // SQUARED CJK UNIFIED IDEOGRAPH-7121 (無)
					baseChar = '無'
				case 0x1F22F: // SQUARED CJK UNIFIED IDEOGRAPH-6307 (指)
					baseChar = '指'
				case 0x1F232: // SQUARED CJK UNIFIED IDEOGRAPH-7981 (禁)
					baseChar = '禁'
				case 0x1F233: // SQUARED CJK UNIFIED IDEOGRAPH-7A7A (空)
					baseChar = '空'
				case 0x1F234: // SQUARED CJK UNIFIED IDEOGRAPH-5408 (合)
					baseChar = '合'
				case 0x1F235: // SQUARED CJK UNIFIED IDEOGRAPH-6E80 (満)
					baseChar = '満'
				case 0x1F236: // SQUARED CJK UNIFIED IDEOGRAPH-6709 (有)
					baseChar = '有'
				case 0x1F237: // SQUARED CJK UNIFIED IDEOGRAPH-6708 (月)
					baseChar = '月'
				case 0x1F238: // SQUARED CJK UNIFIED IDEOGRAPH-7533 (申)
					baseChar = '申'
				case 0x1F239: // SQUARED CJK UNIFIED IDEOGRAPH-5272 (割)
					baseChar = '割'
				case 0x1F23A: // SQUARED CJK UNIFIED IDEOGRAPH-55B6 (営)
					baseChar = '営'
				case 0x1F250: // CIRCLED IDEOGRAPH ADVANTAGE (得)
					baseChar = '得'
				case 0x1F251: // CIRCLED IDEOGRAPH ACCEPT (可)
					baseChar = '可'
				default:
					// その他の囲み漢字はARIBマップに登録済みのものを優先
					// マップにない場合は汎用的表現に変換
					result.WriteString("[囲漢字]")
					continue
				}
				
				// 対応する漢字を[]で囲む
				result.WriteString("[" + string(baseChar) + "]")
			} else {
				// その他の絵文字は「[絵文字]」で代替
				result.WriteString("[絵文字]")
			}
		} else {
			// その他の文字はそのまま
			result.WriteRune(r)
		}
	}

	return result.String()
}

// sanitizeForShiftJIS は任意の文字列をShift-JISエンコード可能な文字に変換する
// エンコードできない文字は近似する文字や代替表現に置き換える
func sanitizeForShiftJIS(s string) string {
	var result strings.Builder
	result.Grow(len(s))

	encoder := japanese.ShiftJIS.NewEncoder()
	
	for _, r := range s {
		// ARIB外字コード範囲はすでに処理済みと想定（normalizeSpecialCharactersで処理）
		// 文字ごとにShift-JISエンコード可能かテストする
		if _, err := encoder.String(string(r)); err == nil {
			// エンコード可能ならそのまま追加
			result.WriteRune(r)
		} else {
			// エンコード不可能な文字を処理
			
			// ARIB外字マップにある文字の場合は対応する変換を使用
			if replacement, ok := models.ARIBGaijiMapAll[r]; ok {
				// 変換後の文字列がShift-JISエンコード可能かテスト
				if _, err := encoder.String(replacement); err == nil {
					result.WriteString(replacement)
					continue
				}
				// エンコード不可能な場合は以降の処理に進む
			}
			
			// Unicode絵文字（U+1F000以降）
			if r >= 0x1F000 {
				result.WriteString("[絵文字]")
			} else if unicode.Is(unicode.Han, r) {
				// その他の漢字（JIS第1・第2水準外の漢字など）
				result.WriteString("[漢字]")
			} else if unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) {
				// 特殊なひらがな・カタカナ
				result.WriteString("[仮名]")
			} else if unicode.IsPunct(r) || unicode.IsSymbol(r) {
				// 記号類
				result.WriteString("・")
			} else {
				// その他のエンコード不可能な文字
				result.WriteString("□")
			}
		}
	}

	return result.String()
}