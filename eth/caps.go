package eth

import (
	"fmt"
	"slices"

	"github.com/ethereum/go-ethereum/p2p"
)

const (
	baseProtocolLength = 16
	protocolLength68   = 17
	protocolLength69   = 18 // eth/69 added BlockRangeUpdateMsg (0x11)
	protocolLength71   = 20 // eth/71 added block access-list messages
	protocolLength72   = 22 // eth/72 added cell messages
)

// Config controls ETH and optional SNAP capability negotiation. MaxVersion is
// zero for the default ETH/70 baseline or 68 through 72. Capabilities supplies
// an exact list and is mutually exclusive with MaxVersion and SnapVersion.
type Config struct {
	MaxVersion   uint
	SnapVersion  uint
	Capabilities []p2p.Cap
}

func (c Config) validate() error {
	if len(c.Capabilities) != 0 && (c.MaxVersion != 0 || c.SnapVersion != 0) {
		return fmt.Errorf("exact capabilities cannot be combined with MaxVersion or SnapVersion")
	}
	if c.MaxVersion != 0 && (c.MaxVersion < 68 || c.MaxVersion > 72) {
		return fmt.Errorf("unsupported ETH max version %d (want 68 through 72)", c.MaxVersion)
	}
	if c.SnapVersion != 0 && c.SnapVersion != 1 && c.SnapVersion != 2 {
		return fmt.Errorf("unsupported SNAP version %d (want 1 or 2)", c.SnapVersion)
	}
	seen := make(map[p2p.Cap]struct{}, len(c.Capabilities))
	for _, capability := range c.Capabilities {
		if !supportedCapability(capability) {
			return fmt.Errorf("unsupported capability %s/%d", capability.Name, capability.Version)
		}
		if _, exists := seen[capability]; exists {
			return fmt.Errorf("duplicate capability %s/%d", capability.Name, capability.Version)
		}
		seen[capability] = struct{}{}
	}
	return nil
}

func supportedCapability(capability p2p.Cap) bool {
	switch capability.Name {
	case "eth":
		return capability.Version >= 68 && capability.Version <= 72
	case "snap":
		return capability.Version == 1 || capability.Version == 2
	case "wit":
		return capability.Version == 1
	default:
		return false
	}
}

func (c Config) capabilities() []p2p.Cap {
	if len(c.Capabilities) != 0 {
		return slices.Clone(c.Capabilities)
	}
	maxVersion := c.MaxVersion
	if maxVersion == 0 {
		maxVersion = 70
	}
	caps := ethCapabilities(maxVersion)
	if c.SnapVersion != 0 {
		caps = append(caps, p2p.Cap{Name: "snap", Version: c.SnapVersion})
	}
	return caps
}

func ethCapabilities(maxVersion uint) []p2p.Cap {
	caps := make([]p2p.Cap, 0, int(maxVersion)-67)
	for version := maxVersion; ; version-- {
		caps = append(caps, p2p.Cap{Name: "eth", Version: version})
		if version == 68 {
			return caps
		}
	}
}

// ProtocolLength returns the negotiated devp2p message-space length for an
// ETH capability version.
func ProtocolLength(version uint) (uint64, error) {
	switch version {
	case 68:
		return protocolLength68, nil
	case 69, 70:
		return protocolLength69, nil
	case 71:
		return protocolLength71, nil
	case 72:
		return protocolLength72, nil
	default:
		return 0, fmt.Errorf("unsupported ETH protocol version %d", version)
	}
}

func negotiatedCapabilities(local, remote []p2p.Cap) ([]p2p.Cap, error) {
	localVersions := make(map[string]map[uint]struct{})
	for _, capability := range local {
		if !supportedCapability(capability) {
			return nil, fmt.Errorf("unsupported local capability %s/%d", capability.Name, capability.Version)
		}
		versions := localVersions[capability.Name]
		if versions == nil {
			versions = make(map[uint]struct{})
			localVersions[capability.Name] = versions
		}
		versions[capability.Version] = struct{}{}
	}

	versions := make(map[string]uint)
	for _, capability := range remote {
		if _, ok := localVersions[capability.Name][capability.Version]; ok && capability.Version > versions[capability.Name] {
			versions[capability.Name] = capability.Version
		}
	}
	names := make([]string, 0, len(versions))
	for name := range versions {
		names = append(names, name)
	}
	slices.Sort(names)

	capabilities := make([]p2p.Cap, 0, len(names))
	for _, name := range names {
		capabilities = append(capabilities, p2p.Cap{Name: name, Version: versions[name]})
	}
	return capabilities, nil
}

// capabilityOffset returns the canonical devp2p wire offset for name.
// Negotiated capability names are ordered lexicographically and only one
// version per name occupies message space.
func capabilityOffset(negotiated []p2p.Cap, name string) (uint64, uint, bool, error) {
	offset := uint64(baseProtocolLength)
	for _, capability := range negotiated {
		if capability.Name == name {
			return offset, capability.Version, true, nil
		}
		length, err := capabilityProtocolLength(capability.Name, capability.Version)
		if err != nil {
			return 0, 0, false, err
		}
		offset += length
	}
	return 0, 0, false, nil
}

func capabilityProtocolLength(name string, version uint) (uint64, error) {
	switch name {
	case "eth":
		return ProtocolLength(version)
	case "snap":
		if version == 1 {
			return 8, nil
		}
		if version == 2 {
			// SNAP/2 reserves 0x06 and 0x07 and adds messages at 0x08 and
			// 0x09, so its static devp2p message space still spans ten IDs.
			return 10, nil
		}
	case "wit":
		if version == 1 {
			return 4, nil
		}
	}
	return 0, fmt.Errorf("unsupported capability %s/%d", name, version)
}
