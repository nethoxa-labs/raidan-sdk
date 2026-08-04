package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ConsensusENR reads the discovery ENR from the standard beacon identity API.
func ConsensusENR(ctx context.Context, beaconURL string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	requestCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, strings.TrimRight(beaconURL, "/")+"/eth/v1/node/identity", nil)
	if err != nil {
		return "", err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("fetch consensus identity: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("fetch consensus identity: HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("read consensus identity: %w", err)
	}
	var identity struct {
		Data struct {
			ENR string `json:"enr"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &identity); err != nil {
		return "", fmt.Errorf("decode consensus identity: %w", err)
	}
	identity.Data.ENR = strings.TrimSpace(identity.Data.ENR)
	if identity.Data.ENR == "" {
		return "", fmt.Errorf("consensus identity returned no ENR")
	}
	return identity.Data.ENR, nil
}
