package rpc

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/forkid"
	"golang.org/x/sync/singleflight"
)

// ChainParams is the execution-chain state required for ETH handshakes.
type ChainParams struct {
	Genesis    common.Hash
	HeadHash   common.Hash
	StateRoot  common.Hash
	NetworkID  uint64
	ForkID     forkid.ID
	HeadNumber uint64   // latest execution block number
	HeadTime   uint64   // latest execution block timestamp
	ForkBlocks []uint64 // ordered block-triggered fork schedule
	ForkTimes  []uint64 // ordered timestamp-triggered fork schedule
}

// chainParamsCache memoizes FetchChainParams per RPC URL. Every access prunes
// all expired URLs, and the hard capacity bounds one-shot ephemeral endpoints.
var chainParamsCache = newChainParamsCache(maxChainParamsCacheEntries)
var chainParamsFetch singleflight.Group

type chainParamsEntry struct {
	cp  ChainParams
	exp time.Time
}

const (
	chainParamsTTL             = 30 * time.Second
	chainParamsFetchTimeout    = 45 * time.Second
	maxChainParamsCacheEntries = 256
)

type chainParamsCacheStore struct {
	mu      sync.Mutex
	entries map[string]chainParamsEntry
	limit   int
}

func newChainParamsCache(limit int) *chainParamsCacheStore {
	if limit < 1 {
		panic("chain params cache limit must be positive")
	}
	return &chainParamsCacheStore{entries: make(map[string]chainParamsEntry), limit: limit}
}

func (cache *chainParamsCacheStore) Load(key string) (chainParamsEntry, bool) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.pruneExpiredLocked(time.Now())
	entry, ok := cache.entries[key]
	return entry, ok
}

func (cache *chainParamsCacheStore) Store(key string, entry chainParamsEntry) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.pruneExpiredLocked(time.Now())
	if _, exists := cache.entries[key]; !exists && len(cache.entries) >= cache.limit {
		cache.evictOldestLocked()
	}
	cache.entries[key] = entry
}

func (cache *chainParamsCacheStore) Clear() {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	clear(cache.entries)
}

func (cache *chainParamsCacheStore) Len() int {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.pruneExpiredLocked(time.Now())
	return len(cache.entries)
}

func (cache *chainParamsCacheStore) pruneExpiredLocked(now time.Time) {
	for key, entry := range cache.entries {
		if !now.Before(entry.exp) {
			delete(cache.entries, key)
		}
	}
}

func (cache *chainParamsCacheStore) evictOldestLocked() {
	var oldestKey string
	var oldestExpiry time.Time
	for key, entry := range cache.entries {
		if oldestKey == "" || entry.exp.Before(oldestExpiry) || entry.exp.Equal(oldestExpiry) && key < oldestKey {
			oldestKey = key
			oldestExpiry = entry.exp
		}
	}
	delete(cache.entries, oldestKey)
}

// FetchChainParams discovers genesis, head, network ID, and the EIP-2124 fork
// ID. It requires admin_nodeInfo to expose the execution chain config. Results
// are cached per RPC URL for chainParamsTTL.
func FetchChainParams(ctx context.Context, rpc string) (ChainParams, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if cached, ok := loadChainParams(rpc); ok {
		return cached, nil
	}
	result := chainParamsFetch.DoChan(rpc, func() (any, error) {
		if cached, ok := loadChainParams(rpc); ok {
			return cached, nil
		}
		// A shared request must not inherit whichever caller happened to win the
		// singleflight race: that caller may have a much shorter deadline than
		// concurrent callers. Keep the shared work independently bounded while
		// every waiter below remains cancellable by its own context.
		fetchCtx, cancel := context.WithTimeout(context.Background(), chainParamsFetchTimeout)
		defer cancel()
		params, err := fetchChainParamsUncached(fetchCtx, rpc)
		if err != nil {
			return ChainParams{}, err
		}
		chainParamsCache.Store(rpc, chainParamsEntry{cp: cloneChainParams(params), exp: time.Now().Add(chainParamsTTL)})
		return params, nil
	})
	select {
	case <-ctx.Done():
		return ChainParams{}, ctx.Err()
	case completed := <-result:
		if completed.Err != nil {
			return ChainParams{}, completed.Err
		}
		return cloneChainParams(completed.Val.(ChainParams)), nil
	}
}

func loadChainParams(rpc string) (ChainParams, bool) {
	entry, ok := chainParamsCache.Load(rpc)
	if !ok {
		return ChainParams{}, false
	}
	return cloneChainParams(entry.cp), true
}

func cloneChainParams(params ChainParams) ChainParams {
	params.ForkBlocks = slices.Clone(params.ForkBlocks)
	params.ForkTimes = slices.Clone(params.ForkTimes)
	return params
}

// Clone returns an independent copy of ChainParams and its fork schedules.
func (params ChainParams) Clone() ChainParams { return cloneChainParams(params) }

func fetchChainParamsUncached(ctx context.Context, rpc string) (ChainParams, error) {
	genesis, err := fetchBlockInfo(ctx, rpc, "0x0")
	if err != nil {
		return ChainParams{}, fmt.Errorf("fetch genesis: %w", err)
	}
	head, err := fetchBlockInfo(ctx, rpc, "latest")
	if err != nil {
		return ChainParams{}, fmt.Errorf("fetch head: %w", err)
	}
	networkID, err := fetchNetworkID(ctx, rpc)
	if err != nil {
		return ChainParams{}, fmt.Errorf("fetch network ID: %w", err)
	}
	schedule, err := fetchForkSchedule(ctx, rpc, genesis.Timestamp)
	if err != nil {
		return ChainParams{}, fmt.Errorf("fetch fork schedule: %w", err)
	}
	forkID := computeForkID(genesis.Hash, head.Number, head.Timestamp, schedule)
	return ChainParams{
		Genesis:    genesis.Hash,
		HeadHash:   head.Hash,
		StateRoot:  head.StateRoot,
		NetworkID:  networkID,
		ForkID:     forkID,
		HeadNumber: head.Number,
		HeadTime:   head.Timestamp,
		ForkBlocks: slices.Clone(schedule.blocks),
		ForkTimes:  slices.Clone(schedule.times),
	}, nil
}

const forkTimestampThreshold = 1438269973

// ValidateForkID applies the EIP-2124 current, subset, superset, and unknown
// future-fork compatibility rules against the fetched local fork schedule.
func (params ChainParams) ValidateForkID(remote forkid.ID) error {
	forks := append(slices.Clone(params.ForkBlocks), params.ForkTimes...)
	sums := make([][4]byte, len(forks)+1)
	hash := crc32.ChecksumIEEE(params.Genesis[:])
	sums[0] = checksumToBytes(hash)
	for i, fork := range forks {
		hash = checksumUpdate(hash, fork)
		sums[i+1] = checksumToBytes(hash)
	}
	blockForkCount := len(params.ForkBlocks)
	forks = append(forks, ^uint64(0))
	if len(params.ForkTimes) == 0 {
		blockForkCount++
	}
	for i, fork := range forks {
		head := params.HeadNumber
		if i >= blockForkCount {
			head = params.HeadTime
		}
		if head >= fork {
			continue
		}
		if sums[i] == remote.Hash {
			if remote.Next > 0 && (head >= remote.Next || remote.Next > forkTimestampThreshold && params.HeadTime >= remote.Next) {
				return fmt.Errorf("remote fork %x/%d is incompatible or stale", remote.Hash, remote.Next)
			}
			return nil
		}
		for j := 0; j < i; j++ {
			if sums[j] == remote.Hash {
				if forks[j] != remote.Next {
					return fmt.Errorf("remote fork %x/%d is stale (expected next %d)", remote.Hash, remote.Next, forks[j])
				}
				return nil
			}
		}
		for j := i + 1; j < len(sums); j++ {
			if sums[j] == remote.Hash {
				return nil
			}
		}
		return fmt.Errorf("remote fork %x/%d is incompatible", remote.Hash, remote.Next)
	}
	return errors.New("fork schedule validation reached an invalid state")
}

type blockInfo struct {
	Hash      common.Hash
	Number    uint64
	Timestamp uint64
	StateRoot common.Hash
}

func fetchBlockInfo(ctx context.Context, rpc, tag string) (blockInfo, error) {
	raw, err := Call(ctx, rpc, "eth_getBlockByNumber", tag, false)
	if err != nil {
		return blockInfo{}, err
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return blockInfo{}, fmt.Errorf("block %s is unavailable", tag)
	}
	var block struct {
		Hash      common.Hash     `json:"hash"`
		Number    json.RawMessage `json:"number"`
		Timestamp json.RawMessage `json:"timestamp"`
		StateRoot common.Hash     `json:"stateRoot"`
	}
	if err := json.Unmarshal(raw, &block); err != nil {
		return blockInfo{}, err
	}
	if block.Hash == (common.Hash{}) {
		return blockInfo{}, fmt.Errorf("block %s has no hash", tag)
	}
	if block.StateRoot == (common.Hash{}) {
		return blockInfo{}, fmt.Errorf("block %s has no state root", tag)
	}
	number, err := parseUint(block.Number)
	if err != nil {
		return blockInfo{}, fmt.Errorf("parse block number %s: %w", block.Number, err)
	}
	timestamp, err := parseUint(block.Timestamp)
	if err != nil {
		return blockInfo{}, fmt.Errorf("parse block timestamp %s: %w", block.Timestamp, err)
	}
	return blockInfo{
		Hash:      block.Hash,
		Number:    number,
		Timestamp: timestamp,
		StateRoot: block.StateRoot,
	}, nil
}

type forkSchedule struct {
	blocks []uint64
	times  []uint64
}

// computeForkID applies EIP-2124's block forks first, then timestamp forks.
func computeForkID(genesisHash common.Hash, headNumber, headTime uint64, schedule forkSchedule) forkid.ID {
	hash := crc32.ChecksumIEEE(genesisHash[:])
	for _, fork := range schedule.blocks {
		if fork > headNumber {
			return forkid.ID{Hash: checksumToBytes(hash), Next: fork}
		}
		hash = checksumUpdate(hash, fork)
	}
	for _, fork := range schedule.times {
		if fork > headTime {
			return forkid.ID{Hash: checksumToBytes(hash), Next: fork}
		}
		hash = checksumUpdate(hash, fork)
	}
	return forkid.ID{Hash: checksumToBytes(hash)}
}

// fetchForkSchedule extracts fork activation blocks and timestamps from the
// config returned by admin_nodeInfo.
func fetchForkSchedule(ctx context.Context, rpc string, genesisTime uint64) (forkSchedule, error) {
	raw, err := Call(ctx, rpc, "admin_nodeInfo")
	if err != nil {
		return forkSchedule{}, err
	}
	var nodeInfo struct {
		Protocols struct {
			Eth struct {
				Config map[string]json.RawMessage `json:"config"`
			} `json:"eth"`
		} `json:"protocols"`
	}
	if err := json.Unmarshal(raw, &nodeInfo); err != nil {
		return forkSchedule{}, err
	}
	if nodeInfo.Protocols.Eth.Config == nil {
		return forkSchedule{}, errors.New("admin_nodeInfo omitted protocols.eth.config")
	}
	return parseForkSchedule(nodeInfo.Protocols.Eth.Config, genesisTime)
}

func parseForkSchedule(config map[string]json.RawMessage, genesisTime uint64) (forkSchedule, error) {
	var schedule forkSchedule
	for key, raw := range config {
		kind := classifyFork(key)
		if kind == forkNone || string(raw) == "null" {
			continue
		}
		fork, err := parseUint(raw)
		if err != nil {
			return forkSchedule{}, fmt.Errorf("parse fork %q: %w", key, err)
		}
		switch kind {
		case forkBlock:
			if fork != 0 {
				schedule.blocks = append(schedule.blocks, fork)
			}
		case forkTime:
			if fork > genesisTime {
				schedule.times = append(schedule.times, fork)
			}
		}
	}
	slices.Sort(schedule.blocks)
	schedule.blocks = slices.Compact(schedule.blocks)
	slices.Sort(schedule.times)
	schedule.times = slices.Compact(schedule.times)
	return schedule, nil
}

type forkKind uint8

const (
	forkNone forkKind = iota
	forkBlock
	forkTime
)

func classifyFork(key string) forkKind {
	key = strings.ToLower(key)
	switch {
	case strings.HasSuffix(key, "transitiontimestamp"), strings.HasSuffix(key, "time"):
		return forkTime
	case strings.HasSuffix(key, "block"), strings.HasSuffix(key, "transition"):
		return forkBlock
	default:
		return forkNone
	}
}

func checksumUpdate(hash uint32, fork uint64) uint32 {
	var blob [8]byte
	binary.BigEndian.PutUint64(blob[:], fork)
	return crc32.Update(hash, crc32.IEEETable, blob[:])
}

func checksumToBytes(hash uint32) [4]byte {
	var blob [4]byte
	binary.BigEndian.PutUint32(blob[:], hash)
	return blob
}

func fetchNetworkID(ctx context.Context, rpc string) (uint64, error) {
	networkID, netErr := fetchRPCUint(ctx, rpc, "net_version")
	if netErr == nil {
		return networkID, nil
	}
	chainID, chainErr := fetchRPCUint(ctx, rpc, "eth_chainId")
	if chainErr == nil {
		return chainID, nil
	}
	return 0, errors.Join(
		fmt.Errorf("net_version: %w", netErr),
		fmt.Errorf("eth_chainId: %w", chainErr),
	)
}

func fetchRPCUint(ctx context.Context, rpc, method string) (uint64, error) {
	raw, err := Call(ctx, rpc, method)
	if err != nil {
		return 0, err
	}
	value, err := parseUint(raw)
	if err != nil {
		return 0, fmt.Errorf("parse %s result %s: %w", method, raw, err)
	}
	return value, nil
}

func parseUint(raw json.RawMessage) (uint64, error) {
	value := strings.TrimSpace(string(raw))
	if strings.HasPrefix(value, `"`) {
		if err := json.Unmarshal(raw, &value); err != nil {
			return 0, err
		}
	}
	base := 10
	if strings.HasPrefix(value, "0x") || strings.HasPrefix(value, "0X") {
		base = 16
		value = value[2:]
	}
	if value == "" {
		return 0, errors.New("empty integer")
	}
	return strconv.ParseUint(value, base, 64)
}
