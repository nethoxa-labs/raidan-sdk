package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/nethoxa-labs/raidan-sdk/session"
)

const (
	rpcCallTimeout      = 10 * time.Second
	maxRPCResponseBytes = 4 << 20
)

var rpcHTTPClient = &http.Client{Timeout: rpcCallTimeout}

// Call performs a bounded JSON-RPC request and returns the raw result.
func Call(ctx context.Context, url, method string, params ...any) (json.RawMessage, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if params == nil {
		params = []any{}
	}
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "method": method, "params": params, "id": 1,
	})
	if err != nil {
		return nil, fmt.Errorf("encode rpc request: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, session.Timeout(ctx, rpcCallTimeout))
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := rpcHTTPClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer func() { _ = response.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(response.Body, maxRPCResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxRPCResponseBytes {
		return nil, fmt.Errorf("rpc response exceeds %d bytes", maxRPCResponseBytes)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		message := strings.TrimSpace(string(data))
		if len(message) > 512 {
			message = message[:512]
		}
		return nil, fmt.Errorf("rpc HTTP %s: %s", response.Status, message)
	}
	var result struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Result  json.RawMessage `json:"result"`
		Error   *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&result); err != nil {
		return nil, fmt.Errorf("json: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, errors.New("rpc response must contain exactly one JSON object")
	}
	if result.JSONRPC != "2.0" || string(result.ID) != "1" {
		return nil, errors.New("rpc response does not match the request")
	}
	if (result.Error == nil) == (result.Result == nil) {
		return nil, errors.New("rpc response must contain exactly one of result or error")
	}
	if result.Error != nil {
		return nil, fmt.Errorf("rpc %d: %s", result.Error.Code, result.Error.Message)
	}
	return result.Result, nil
}
