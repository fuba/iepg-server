// handlers/jsonrpc.go
package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/fuba/iepg-server/db"
	"github.com/fuba/iepg-server/models"
)

// JSONRPCRequest は JSON-RPC 2.0 のリクエストフォーマット
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
	ID      interface{}     `json:"id"`
}

// JSONRPCResponse は JSON-RPC 2.0 のレスポンスフォーマット
type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
	ID      interface{} `json:"id"`
}

// RPCError は JSON-RPC のエラー構造体
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// SearchParams は RPC 用の検索パラメータ
type SearchParams struct {
	Q         string `json:"q"`
	ServiceID int64  `json:"serviceId"`
	StartFrom int64  `json:"startFrom"`
	StartTo   int64  `json:"startTo"`
}

// rpcHandler は JSON-RPC リクエストのディスパッチ処理を行う
func rpcHandler(dbConn *sql.DB, w http.ResponseWriter, r *http.Request) {
	models.Log.Debug("rpcHandler: Processing JSON-RPC request from %s", r.RemoteAddr)
	
	var req JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		models.Log.Error("rpcHandler: Failed to parse JSON-RPC request: %v", err)
		writeRPCError(w, nil, -32700, "Parse error")
		return
	}
	
	models.Log.Debug("rpcHandler: Received request - Method: %s, JSONRPC: %s, ID: %v", 
		req.Method, req.JSONRPC, req.ID)
	
	if req.JSONRPC != "2.0" {
		models.Log.Error("rpcHandler: Invalid JSON-RPC version: %s", req.JSONRPC)
		writeRPCError(w, req.ID, -32600, "Invalid Request")
		return
	}
	
	switch req.Method {
	case "searchPrograms":
		models.Log.Debug("rpcHandler: Handling searchPrograms method")
		
		var params SearchParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			models.Log.Error("rpcHandler: Invalid params for searchPrograms: %v", err)
			writeRPCError(w, req.ID, -32602, "Invalid params")
			return
		}
		
		models.Log.Debug("rpcHandler: searchPrograms params - q=%s, serviceId=%d, startFrom=%d, startTo=%d", 
			params.Q, params.ServiceID, params.StartFrom, params.StartTo)
		
		result, err := searchProgramsRPC(dbConn, params)
		if err != nil {
			models.Log.Error("rpcHandler: searchPrograms failed: %v", err)
			writeRPCError(w, req.ID, -32000, err.Error())
			return
		}
		
		models.Log.Info("rpcHandler: searchPrograms completed, found %d programs", len(result))
		
		resp := JSONRPCResponse{
			JSONRPC: "2.0",
			Result:  result,
			ID:      req.ID,
		}
		
		models.Log.Debug("rpcHandler: Sending successful JSON-RPC response")
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			models.Log.Error("rpcHandler: Failed to encode JSON response: %v", err)
		}
		
	default:
		models.Log.Error("rpcHandler: Method not found: %s", req.Method)
		writeRPCError(w, req.ID, -32601, "Method not found")
	}
}

func writeRPCError(w http.ResponseWriter, id interface{}, code int, message string) {
	models.Log.Debug("writeRPCError: Writing RPC error - Code: %d, Message: %s", code, message)
	
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		Error: &RPCError{
			Code:    code,
			Message: message,
		},
		ID: id,
	}
	
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		models.Log.Error("writeRPCError: Failed to encode error response: %v", err)
	}
}

// searchProgramsRPC は SearchParams をもとにプログラムを検索する
func searchProgramsRPC(dbConn *sql.DB, params SearchParams) ([]models.Program, error) {
	models.Log.Debug("searchProgramsRPC: Searching programs with params - q=%s, serviceId=%d, startFrom=%d, startTo=%d", 
		params.Q, params.ServiceID, params.StartFrom, params.StartTo)
	
	return db.SearchPrograms(dbConn, params.Q, params.ServiceID, params.StartFrom, params.StartTo)
}

// NewRPCServer は JSON-RPC 用のHTTPハンドラを返す
func NewRPCServer(dbConn *sql.DB) http.Handler {
	models.Log.Debug("NewRPCServer: Creating new JSON-RPC server handler")
	
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		models.Log.Debug("RPCServer: Received %s request from %s", r.Method, r.RemoteAddr)
		
		if r.Method != "POST" {
			models.Log.Error("RPCServer: Method not allowed: %s", r.Method)
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		
		rpcHandler(dbConn, w, r)
	})
}