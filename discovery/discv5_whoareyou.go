package discovery

import (
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"github.com/nethoxa-labs/raidan-sdk/session"
)

// WhoAreYou is the parsed result of the recipient's challenge.
type WhoAreYou struct {
	ChallengeData []byte   // raw bytes: masking-iv || static-header || authdata (UNMASKED)
	IDNonce       [16]byte // first 16 bytes of authdata
	RecordSeq     uint64   // last 8 bytes of authdata
	Nonce         [12]byte // packet nonce (echoes our request nonce)
}

// ReadWhoAreYou waits up to timeout for a WHOAREYOU packet, unmasks it
// using the local node ID as the destination masking key. ChallengeData is the
// input to the identity signature: masking IV, unmasked static header, and
// unmasked authentication data.
func (s *Discv5Conn) ReadWhoAreYou(timeout time.Duration) (*WhoAreYou, error) {
	return s.readWhoAreYou(timeout, nil)
}

func (s *Discv5Conn) readWhoAreYou(timeout time.Duration, expectedNonce *[12]byte) (*WhoAreYou, error) {
	deadline := time.Now().Add(session.Timeout(s.ctx, timeout))
	buf := make([]byte, 1280)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, errors.New("timeout")
		}
		n, from, err := readUDPWithContext(s.ctx, s.fd, buf, remaining)
		if err != nil {
			return nil, err
		}
		if !SameUDPAddr(from, s.peerAddr) {
			continue
		}
		challenge, err := s.decodeWhoAreYou(buf[:n])
		if err != nil {
			return nil, err
		}
		if expectedNonce != nil && challenge.Nonce != *expectedNonce {
			continue
		}
		return challenge, nil
	}
}

func (s *Discv5Conn) decodeWhoAreYou(packet []byte) (*WhoAreYou, error) {
	if len(packet) != discv5MinPacketSize {
		return nil, fmt.Errorf("WHOAREYOU packet is %d bytes, want %d", len(packet), discv5MinPacketSize)
	}

	var iv [16]byte
	copy(iv[:], packet[:16])

	// Unmask static-header + authdata. WHOAREYOU authdata is 24 bytes
	// (16 idnonce + 8 record-seq).
	maskedHeader := packet[16 : 16+discv5StaticHeaderSize+24]
	unmasked := make([]byte, len(maskedHeader))
	applyMaskCTR(s.ourNodeID, iv, unmasked, maskedHeader)

	staticHdr := unmasked[:discv5StaticHeaderSize]
	authdata := unmasked[discv5StaticHeaderSize:]

	if string(staticHdr[0:6]) != string(discv5DefaultProtoID[:]) {
		return nil, errors.New("unexpected protocol-id in reply")
	}
	if version := binary.BigEndian.Uint16(staticHdr[6:8]); version != Discv5Version {
		return nil, fmt.Errorf("unexpected discv5 version 0x%04x", version)
	}
	if staticHdr[8] != Discv5FlagWhoAreYou {
		return nil, fmt.Errorf("expected WHOAREYOU flag (0x01), got 0x%02x", staticHdr[8])
	}
	authSize := binary.BigEndian.Uint16(staticHdr[21:23])
	if int(authSize) != len(authdata) {
		return nil, fmt.Errorf("authdata size mismatch: hdr=%d, got=%d", authSize, len(authdata))
	}

	w := &WhoAreYou{
		ChallengeData: append(append([]byte{}, iv[:]...), unmasked...),
	}
	copy(w.Nonce[:], staticHdr[9:21])
	copy(w.IDNonce[:], authdata[:16])
	w.RecordSeq = binary.BigEndian.Uint64(authdata[16:24])
	return w, nil
}
