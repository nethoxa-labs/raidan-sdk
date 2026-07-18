package reqresp

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/nethoxa-labs/raidan-sdk/session"
)

const (
	// StatusV1 is the consensus Status v1 protocol ID.
	StatusV1 = ProtocolPrefix + "/status/1/" + Encoding
	// StatusV2 is the consensus Status v2 protocol ID.
	StatusV2              = ProtocolPrefix + "/status/2/" + Encoding
	maxConsensusJSONBytes = 1 << 20
)

// StatusWarmup completes a Status v2 handshake, falling back to Status v1.
func (s *Session) StatusWarmup(beaconURL string) error {
	statusV1, statusV2, err := BeaconStatus(s.ctx, beaconURL)
	if err != nil {
		return err
	}
	if response, err := s.Request(StatusV2, statusV2, RequestOptions{}); err == nil && response.Code == CodeSuccess {
		return nil
	}
	response, err := s.Request(StatusV1, statusV1, RequestOptions{})
	if err != nil {
		return err
	}
	if response.Code != CodeSuccess {
		return fmt.Errorf("status warmup returned response code %d", response.Code)
	}
	return nil
}

// BeaconStatus builds the canonical Status v1 and v2 request bodies.
func BeaconStatus(ctx context.Context, beaconURL string) ([]byte, []byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	var forkResponse struct {
		Data struct {
			CurrentVersion string `json:"current_version"`
		} `json:"data"`
	}
	if err := consensusGetJSON(ctx, beaconURL, "/eth/v1/beacon/states/head/fork", &forkResponse); err != nil {
		return nil, nil, fmt.Errorf("fetch fork: %w", err)
	}
	currentVersion, err := consensusHex(forkResponse.Data.CurrentVersion, 4)
	if err != nil {
		return nil, nil, fmt.Errorf("current_version: %w", err)
	}

	var genesisResponse struct {
		Data struct {
			GenesisValidatorsRoot string `json:"genesis_validators_root"`
		} `json:"data"`
	}
	if err := consensusGetJSON(ctx, beaconURL, "/eth/v1/beacon/genesis", &genesisResponse); err != nil {
		return nil, nil, fmt.Errorf("fetch genesis: %w", err)
	}
	genesisRoot, err := consensusHex(genesisResponse.Data.GenesisValidatorsRoot, 32)
	if err != nil {
		return nil, nil, fmt.Errorf("genesis_validators_root: %w", err)
	}

	var headerResponse struct {
		Data struct {
			Root   string `json:"root"`
			Header struct {
				Message struct {
					Slot string `json:"slot"`
				} `json:"message"`
			} `json:"header"`
		} `json:"data"`
	}
	if err := consensusGetJSON(ctx, beaconURL, "/eth/v1/beacon/headers/head", &headerResponse); err != nil {
		return nil, nil, fmt.Errorf("fetch head header: %w", err)
	}
	headRoot, err := consensusHex(headerResponse.Data.Root, 32)
	if err != nil {
		return nil, nil, fmt.Errorf("head root: %w", err)
	}
	headSlot, err := strconv.ParseUint(headerResponse.Data.Header.Message.Slot, 10, 64)
	if err != nil {
		return nil, nil, fmt.Errorf("head slot: %w", err)
	}

	var finalityResponse struct {
		Data struct {
			Finalized struct {
				Root  string `json:"root"`
				Epoch string `json:"epoch"`
			} `json:"finalized"`
		} `json:"data"`
	}
	if err := consensusGetJSON(ctx, beaconURL, "/eth/v1/beacon/states/head/finality_checkpoints", &finalityResponse); err != nil {
		return nil, nil, fmt.Errorf("fetch finality: %w", err)
	}
	finalizedRoot, err := consensusHex(finalityResponse.Data.Finalized.Root, 32)
	if err != nil {
		return nil, nil, fmt.Errorf("finalized root: %w", err)
	}
	finalizedEpoch, err := strconv.ParseUint(finalityResponse.Data.Finalized.Epoch, 10, 64)
	if err != nil {
		return nil, nil, fmt.Errorf("finalized epoch: %w", err)
	}

	forkDigest := consensusForkDigest(currentVersion, genesisRoot)
	v1 := make([]byte, 84)
	copy(v1[0:4], forkDigest[:])
	copy(v1[4:36], finalizedRoot)
	binary.LittleEndian.PutUint64(v1[36:44], finalizedEpoch)
	copy(v1[44:76], headRoot)
	binary.LittleEndian.PutUint64(v1[76:84], headSlot)
	v2 := make([]byte, 92)
	copy(v2, v1)
	return v1, v2, nil
}

func consensusForkDigest(currentVersion, genesisValidatorsRoot []byte) [4]byte {
	var versionChunk, rootChunk [32]byte
	copy(versionChunk[:], currentVersion)
	copy(rootChunk[:], genesisValidatorsRoot)
	root := sha256.Sum256(append(versionChunk[:], rootChunk[:]...))
	return [4]byte(root[:4])
}

func consensusGetJSON(ctx context.Context, baseURL, path string, output any) error {
	if ctx == nil {
		ctx = context.Background()
	}
	endpoint, err := url.Parse(baseURL)
	if err != nil {
		return err
	}
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + path
	timeout := session.Timeout(ctx, 5*time.Second)
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return err
	}
	client := http.Client{Timeout: timeout}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return fmt.Errorf("http %d from %s", response.StatusCode, endpoint)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxConsensusJSONBytes+1))
	if err != nil {
		return fmt.Errorf("read JSON response: %w", err)
	}
	if len(body) > maxConsensusJSONBytes {
		return fmt.Errorf("JSON response exceeds %d bytes", maxConsensusJSONBytes)
	}
	if len(body) == 0 {
		return fmt.Errorf("JSON response is empty")
	}
	if err := json.Unmarshal(body, output); err != nil {
		return fmt.Errorf("decode JSON response: %w", err)
	}
	return nil
}

func consensusHex(value string, want int) ([]byte, error) {
	decoded, err := hex.DecodeString(strings.TrimPrefix(value, "0x"))
	if err != nil {
		return nil, err
	}
	if len(decoded) != want {
		return nil, fmt.Errorf("got %d bytes, want %d", len(decoded), want)
	}
	return decoded, nil
}
