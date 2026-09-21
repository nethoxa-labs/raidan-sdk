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
//
// Fulu clients drop Status v1, so the fallback usually fails with "protocol not
// supported" or with a closed connection. That message hides why v2 failed, so
// both attempts are reported when neither succeeds.
func (s *Session) StatusWarmup(beaconURL string) error {
	statusV1, statusV2, err := BeaconStatus(s.ctx, beaconURL)
	if err != nil {
		return err
	}
	responseV2, errV2 := s.Request(StatusV2, statusV2, RequestOptions{})
	if errV2 == nil && responseV2.Code == CodeSuccess {
		return nil
	}
	if errV2 == nil {
		errV2 = fmt.Errorf("response code %d", responseV2.Code)
	}
	responseV1, errV1 := s.Request(StatusV1, statusV1, RequestOptions{})
	if errV1 == nil && responseV1.Code == CodeSuccess {
		return nil
	}
	if errV1 == nil {
		errV1 = fmt.Errorf("response code %d", responseV1.Code)
	}
	return fmt.Errorf("status warmup failed: v2: %w; v1: %v", errV2, errV1)
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

	forkDigest, err := consensusForkDigest(ctx, beaconURL, currentVersion, genesisRoot, headSlot)
	if err != nil {
		return nil, nil, err
	}
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

// consensusForkDigest returns the 4-byte fork digest the network uses at the
// epoch containing headSlot.
//
// Before Fulu the digest is the first four bytes of the ForkData root. Fulu's
// EIP-7892 blob-parameter-only forks change the digest without changing the
// fork version, so from FULU_FORK_EPOCH onwards the ForkData root is masked
// with the hash of the active blob parameters. A peer that keeps using the
// unmasked digest presents an unknown fork to every Fulu client and its Status
// handshake and gossip topics are rejected.
func consensusForkDigest(
	ctx context.Context,
	beaconURL string,
	currentVersion, genesisValidatorsRoot []byte,
	headSlot uint64,
) ([4]byte, error) {
	var versionChunk, rootChunk [32]byte
	copy(versionChunk[:], currentVersion)
	copy(rootChunk[:], genesisValidatorsRoot)
	base := sha256.Sum256(append(versionChunk[:], rootChunk[:]...))

	config, err := consensusConfig(ctx, beaconURL)
	if err != nil {
		// An endpoint without a config spec cannot be a Fulu network this SDK
		// can describe; keep the pre-Fulu digest rather than failing the case.
		return [4]byte(base[:4]), nil
	}
	fuluEpoch, ok := config.uint("FULU_FORK_EPOCH")
	if !ok {
		return [4]byte(base[:4]), nil
	}
	slotsPerEpoch, ok := config.uint("SLOTS_PER_EPOCH")
	if !ok || slotsPerEpoch == 0 {
		return [4]byte{}, fmt.Errorf("config spec has no usable SLOTS_PER_EPOCH")
	}
	epoch := headSlot / slotsPerEpoch
	if epoch < fuluEpoch {
		return [4]byte(base[:4]), nil
	}

	blobEpoch, maxBlobs := config.blobParameters(epoch)
	var mask [16]byte
	binary.LittleEndian.PutUint64(mask[0:8], blobEpoch)
	binary.LittleEndian.PutUint64(mask[8:16], maxBlobs)
	masked := sha256.Sum256(mask[:])
	var digest [4]byte
	for index := range digest {
		digest[index] = base[index] ^ masked[index]
	}
	return digest, nil
}

// consensusSpec is the subset of /eth/v1/config/spec this SDK needs. Every
// scalar arrives as a decimal string.
type consensusSpec struct {
	values   map[string]string
	schedule []blobScheduleEntry
}

type blobScheduleEntry struct {
	Epoch    uint64
	MaxBlobs uint64
}

func (c consensusSpec) uint(key string) (uint64, bool) {
	raw, ok := c.values[key]
	if !ok {
		return 0, false
	}
	value, err := strconv.ParseUint(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return 0, false
	}
	return value, true
}

// blobParameters mirrors get_blob_parameters: the newest BLOB_SCHEDULE entry at
// or below epoch, falling back to the Electra defaults.
func (c consensusSpec) blobParameters(epoch uint64) (uint64, uint64) {
	best := blobScheduleEntry{}
	found := false
	for _, entry := range c.schedule {
		if entry.Epoch <= epoch && (!found || entry.Epoch > best.Epoch) {
			best, found = entry, true
		}
	}
	if found {
		return best.Epoch, best.MaxBlobs
	}
	electraEpoch, _ := c.uint("ELECTRA_FORK_EPOCH")
	maxBlobs, _ := c.uint("MAX_BLOBS_PER_BLOCK_ELECTRA")
	return electraEpoch, maxBlobs
}

func consensusConfig(ctx context.Context, beaconURL string) (consensusSpec, error) {
	var payload struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	if err := consensusGetJSON(ctx, beaconURL, "/eth/v1/config/spec", &payload); err != nil {
		return consensusSpec{}, err
	}
	spec := consensusSpec{values: make(map[string]string, len(payload.Data))}
	for key, raw := range payload.Data {
		var text string
		if err := json.Unmarshal(raw, &text); err == nil {
			spec.values[key] = text
			continue
		}
		if key != "BLOB_SCHEDULE" {
			continue
		}
		var entries []struct {
			Epoch    string `json:"EPOCH"`
			MaxBlobs string `json:"MAX_BLOBS_PER_BLOCK"`
		}
		if err := json.Unmarshal(raw, &entries); err != nil {
			continue
		}
		for _, entry := range entries {
			epoch, epochErr := strconv.ParseUint(strings.TrimSpace(entry.Epoch), 10, 64)
			maxBlobs, blobErr := strconv.ParseUint(strings.TrimSpace(entry.MaxBlobs), 10, 64)
			if epochErr != nil || blobErr != nil {
				continue
			}
			spec.schedule = append(spec.schedule, blobScheduleEntry{Epoch: epoch, MaxBlobs: maxBlobs})
		}
	}
	return spec, nil
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
	timeout := session.Remaining(ctx, 5*time.Second)
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
