package discovery

// Helpers for the DNS-based node discovery wire format
// (https://github.com/ethereum/devp2p/blob/master/dnsdisc.md).

import (
	"encoding/base32"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
)

const dnsdiscMaxBranchChildren = 13
const dnsdiscMaxTXTRecordSize = 512
const dnsdiscMinHashBytes = 12
const dnsdiscMaxHashBytes = 32

var dnsdiscBase32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// ValidateDNSRoot applies the minimal rules from EIP-1459 §3
// (https://github.com/ethereum/devp2p/blob/master/dnsdisc.md#dns-record-structure). A real client implements more;
// only the format preamble is checked here.
func ValidateDNSRoot(entry string) error {
	if entry == "" {
		return fmt.Errorf("empty entry")
	}
	if len(entry) > dnsdiscMaxTXTRecordSize {
		return fmt.Errorf("TXT record is %d bytes, max %d", len(entry), dnsdiscMaxTXTRecordSize)
	}
	fields := strings.Fields(entry)
	if len(fields) != 5 {
		return fmt.Errorf("root has %d fields, want 5", len(fields))
	}
	if fields[0] != "enrtree-root:v1" {
		return fmt.Errorf("missing 'enrtree-root:v1' prefix")
	}
	e, err := dnsdiscRequiredField(fields[1], "e=")
	if err != nil {
		return err
	}
	l, err := dnsdiscRequiredField(fields[2], "l=")
	if err != nil {
		return err
	}
	seq, err := dnsdiscRequiredField(fields[3], "seq=")
	if err != nil {
		return err
	}
	sig, err := dnsdiscRequiredField(fields[4], "sig=")
	if err != nil {
		return err
	}
	if _, err := decodeDnsdiscHashField(e); err != nil {
		return fmt.Errorf("invalid e root hash: %w", err)
	}
	if _, err := decodeDnsdiscHashField(l); err != nil {
		return fmt.Errorf("invalid l root hash: %w", err)
	}
	if seq == "" {
		return fmt.Errorf("empty seq")
	}
	if _, err := strconv.ParseUint(seq, 10, 64); err != nil {
		return fmt.Errorf("invalid seq: %w", err)
	}
	if sig == "" {
		return fmt.Errorf("empty sig")
	}
	sigBytes, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil {
		return fmt.Errorf("invalid sig encoding: %w", err)
	}
	if len(sigBytes) != 65 {
		return fmt.Errorf("sig is %d bytes, want 65", len(sigBytes))
	}
	return nil
}

func dnsdiscRequiredField(token, prefix string) (string, error) {
	if !strings.HasPrefix(token, prefix) {
		return "", fmt.Errorf("missing field %q", prefix)
	}
	return strings.TrimPrefix(token, prefix), nil
}

// ValidateDNSBranch applies the branch fanout bound used by the
// discovery tree builders. Runtime resolvers must also track visited
// hashes/domains; this helper only checks one TXT entry's shape.
func ValidateDNSBranch(entry string) error {
	const prefix = "enrtree-branch:"
	if !strings.HasPrefix(entry, prefix) {
		return fmt.Errorf("missing %q prefix", prefix)
	}
	body := strings.TrimPrefix(entry, prefix)
	if body == "" {
		return nil
	}
	children := strings.Split(body, ",")
	if len(children) > dnsdiscMaxBranchChildren {
		return fmt.Errorf("branch has %d children, max %d", len(children), dnsdiscMaxBranchChildren)
	}
	for _, child := range children {
		if child == "" {
			return fmt.Errorf("empty branch child")
		}
		if _, err := decodeDnsdiscHashField(child); err != nil {
			return fmt.Errorf("invalid branch child %q: %w", child, err)
		}
	}
	return nil
}

// ValidateDNSSubtreeEntry checks whether a resolved TXT entry is valid for
// the root-selected subtree kind. ENR leaves belong only below e=, while
// enrtree:// links belong only below l=.
func ValidateDNSSubtreeEntry(entry string, linkTree bool) error {
	switch {
	case strings.HasPrefix(entry, "enrtree-branch:"):
		return ValidateDNSBranch(entry)
	case strings.HasPrefix(entry, "enr:"):
		if linkTree {
			return fmt.Errorf("enr entry in link tree")
		}
		if entry == "enr:" {
			return fmt.Errorf("empty enr leaf")
		}
		return nil
	case strings.HasPrefix(entry, "enrtree://"):
		if !linkTree {
			return fmt.Errorf("link entry in ENR tree")
		}
		return ValidateDNSLink(entry)
	default:
		return fmt.Errorf("unknown DNS discovery entry")
	}
}

// ValidateDNSBranchWalk checks a static branch graph for traversal
// hazards a live resolver must guard against when recursively following
// TXT subdomains.
func ValidateDNSBranchWalk(entries map[string]string, root string, maxDepth int) error {
	visiting := make(map[string]bool)
	visited := make(map[string]bool)
	var walk func(string, int) error
	walk = func(name string, depth int) error {
		if depth > maxDepth {
			return fmt.Errorf("branch depth exceeds %d", maxDepth)
		}
		if visiting[name] {
			return fmt.Errorf("branch cycle at %q", name)
		}
		if visited[name] {
			return nil
		}
		entry, ok := entries[name]
		if !ok {
			return fmt.Errorf("missing branch %q", name)
		}
		if err := ValidateDNSBranch(entry); err != nil {
			return err
		}
		visiting[name] = true
		defer delete(visiting, name)

		body := strings.TrimPrefix(entry, "enrtree-branch:")
		if body == "" {
			return nil
		}
		for _, child := range strings.Split(body, ",") {
			if err := walk(child, depth+1); err != nil {
				return err
			}
		}
		visited[name] = true
		return nil
	}
	return walk(root, 0)
}

// ValidateDNSLink applies the minimal scheme/syntax check for links
// per https://github.com/ethereum/devp2p/blob/master/dnsdisc.md#url-scheme.
func ValidateDNSLink(entry string) error {
	if !strings.HasPrefix(entry, "enrtree://") {
		return fmt.Errorf("missing 'enrtree://' scheme")
	}
	at := strings.Index(entry, "@")
	if at < 0 {
		return fmt.Errorf("missing '@'")
	}
	key := entry[len("enrtree://"):at]
	if len(key) == 0 {
		return fmt.Errorf("empty key")
	}
	keyBytes, err := decodeDnsdiscBase32Field(key)
	if err != nil {
		return fmt.Errorf("invalid key encoding: %w", err)
	}
	if len(keyBytes) != 33 {
		return fmt.Errorf("key is %d bytes, want compressed secp256k1 pubkey", len(keyBytes))
	}
	if _, err := crypto.DecompressPubkey(keyBytes); err != nil {
		return fmt.Errorf("invalid secp256k1 pubkey: %w", err)
	}
	domain := entry[at+1:]
	if len(domain) == 0 {
		return fmt.Errorf("empty domain")
	}
	return nil
}

func decodeDnsdiscBase32Field(value string) ([]byte, error) {
	if value == "" {
		return nil, fmt.Errorf("empty base32 value")
	}
	decoded, err := dnsdiscBase32.DecodeString(strings.ToUpper(value))
	if err != nil {
		return nil, err
	}
	if len(decoded) == 0 {
		return nil, fmt.Errorf("empty decoded value")
	}
	return decoded, nil
}

func decodeDnsdiscHashField(value string) ([]byte, error) {
	decoded, err := decodeDnsdiscBase32Field(value)
	if err != nil {
		return nil, err
	}
	if len(decoded) < dnsdiscMinHashBytes || len(decoded) > dnsdiscMaxHashBytes {
		return nil, fmt.Errorf("decoded hash is %d bytes, want %d..%d", len(decoded), dnsdiscMinHashBytes, dnsdiscMaxHashBytes)
	}
	return decoded, nil
}
