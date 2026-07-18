package target

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const discoveryTimeout = 15 * time.Second

var discoveryHTTPClient = &http.Client{Timeout: discoveryTimeout}

// Target describes one reachable Ethereum network. ExecutionURL is required;
// missing peer endpoints can be discovered from the HTTP APIs.
type Target struct {
	ExecutionURL string
	ExecutionP2P string
	ConsensusURL string
	ConsensusP2P string
	Client       string
}

// Discover validates spec, normalizes its endpoints, and fills missing peer
// addresses from the local node APIs. Explicit fields always win.
func Discover(ctx context.Context, spec Target) (Target, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	spec.ExecutionURL = strings.TrimRight(strings.TrimSpace(spec.ExecutionURL), "/")
	spec.ExecutionP2P = strings.TrimSpace(spec.ExecutionP2P)
	spec.ConsensusURL = strings.TrimRight(strings.TrimSpace(spec.ConsensusURL), "/")
	spec.ConsensusP2P = strings.TrimSpace(spec.ConsensusP2P)
	spec.Client = strings.TrimSpace(spec.Client)
	if err := validateHTTPURL(spec.ExecutionURL); err != nil {
		return Target{}, fmt.Errorf("execution endpoint: %w", err)
	}
	if spec.ConsensusURL != "" {
		if err := validateHTTPURL(spec.ConsensusURL); err != nil {
			return Target{}, fmt.Errorf("consensus endpoint: %w", err)
		}
	}
	if spec.ExecutionP2P == "" {
		endpoint, err := discoverExecutionP2P(ctx, spec.ExecutionURL)
		if err != nil {
			return Target{}, err
		}
		spec.ExecutionP2P = endpoint
	}
	if spec.ConsensusURL != "" && spec.ConsensusP2P == "" {
		endpoint, err := discoverConsensusP2P(ctx, spec.ConsensusURL)
		if err != nil {
			return Target{}, err
		}
		spec.ConsensusP2P = endpoint
	}
	return spec, nil
}

func discoverExecutionP2P(ctx context.Context, endpoint string) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "admin_nodeInfo", "params": []any{},
	})
	var response struct {
		Result struct {
			Enode string `json:"enode"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := requestJSON(ctx, http.MethodPost, endpoint, bytes.NewReader(body), &response); err != nil {
		return "", fmt.Errorf("discover execution peer: %w", err)
	}
	if response.Error != nil {
		return "", fmt.Errorf("discover execution peer: %s", response.Error.Message)
	}
	if strings.TrimSpace(response.Result.Enode) == "" {
		return "", errors.New("discover execution peer: admin_nodeInfo returned no enode")
	}
	return response.Result.Enode, nil
}

func discoverConsensusP2P(ctx context.Context, endpoint string) (string, error) {
	var response struct {
		Data struct {
			ENR          string   `json:"enr"`
			P2PAddresses []string `json:"p2p_addresses"`
		} `json:"data"`
	}
	url := strings.TrimRight(endpoint, "/") + "/eth/v1/node/identity"
	if err := requestJSON(ctx, http.MethodGet, url, nil, &response); err != nil {
		return "", fmt.Errorf("discover consensus peer: %w", err)
	}
	for _, address := range response.Data.P2PAddresses {
		if address = strings.TrimSpace(address); address != "" {
			return address, nil
		}
	}
	if enr := strings.TrimSpace(response.Data.ENR); enr != "" {
		return enr, nil
	}
	return "", errors.New("discover consensus peer: identity returned no ENR or P2P address")
}

func requestJSON(ctx context.Context, method, endpoint string, body io.Reader, output any) error {
	request, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := discoveryHTTPClient.Do(request)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d", response.StatusCode)
	}
	const maxResponseBytes = 1 << 20
	payload, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return err
	}
	if len(payload) > maxResponseBytes {
		return fmt.Errorf("response exceeds %d bytes", maxResponseBytes)
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	if err := decoder.Decode(output); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("response must contain exactly one JSON value")
	}
	return nil
}

func validateHTTPURL(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("URL must use http or https")
	}
	if parsed.Host == "" {
		return errors.New("URL must include a host")
	}
	return nil
}
