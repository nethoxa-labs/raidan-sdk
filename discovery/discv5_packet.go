package discovery

// Discv5 packet helpers per
// https://github.com/ethereum/devp2p/blob/master/discv5/discv5-wire.md#packet-encoding.

import (
	"crypto/aes"
	"crypto/cipher"
	crand "crypto/rand"
	"encoding/binary"
	"fmt"
	"math"

	"github.com/ethereum/go-ethereum/p2p/enode"

	"github.com/nethoxa-labs/raidan-sdk/session"
)

// applyMaskCTR XOR-masks src into dst with AES-128-CTR using key=destID[:16].
func applyMaskCTR(destID enode.ID, iv [16]byte, dst, src []byte) {
	block, err := aes.NewCipher(destID[:16])
	if err != nil {
		panic("aes cipher: " + err.Error())
	}
	cipher.NewCTR(block, iv[:]).XORKeyStream(dst, src)
}

// MaskDiscv5Header returns the AES-CTR-masked form of an unmasked discv5
// static header and authentication data for destinationID.
func MaskDiscv5Header(destinationID enode.ID, iv [16]byte, header []byte) []byte {
	masked := make([]byte, len(header))
	applyMaskCTR(destinationID, iv, masked, header)
	return masked
}

// SendRandomOrdinary sends a flag=0x00 (ordinary) packet whose body is
// uniformly random bytes. The recipient cannot decrypt it (no session)
// and replies with WHOAREYOU. Returns the 12-byte packet nonce we sent —
// the recipient echoes it inside the WHOAREYOU's static-header.
func (s *Discv5Conn) SendRandomOrdinary() ([12]byte, error) {
	var nonce [12]byte
	if _, err := crand.Read(nonce[:]); err != nil {
		return nonce, err
	}
	// authdata = src-id (32 bytes for ordinary)
	authdata := s.ourNodeID[:]
	authsize := uint16(len(authdata))

	// static-header.
	staticHeader := make([]byte, discv5StaticHeaderSize)
	copy(staticHeader[0:6], discv5DefaultProtoID[:])
	binary.BigEndian.PutUint16(staticHeader[6:8], Discv5Version)
	staticHeader[8] = Discv5FlagOrdinary
	copy(staticHeader[9:21], nonce[:])
	binary.BigEndian.PutUint16(staticHeader[21:23], authsize)

	// 20-byte random message body — recipient drops it but counts the
	// packet as "from an unknown peer" and replies with WHOAREYOU.
	msg := make([]byte, 20)
	if _, err := crand.Read(msg); err != nil {
		return nonce, err
	}

	var iv [16]byte
	if _, err := crand.Read(iv[:]); err != nil {
		return nonce, err
	}

	// Mask static-header + authdata.
	plain := append(staticHeader, authdata...)
	masked := make([]byte, len(plain))
	applyMaskCTR(s.peerNodeID, iv, masked, plain)

	pkt := make([]byte, 0, len(iv)+len(masked)+len(msg))
	pkt = append(pkt, iv[:]...)
	pkt = append(pkt, masked...)
	pkt = append(pkt, msg...)

	_, err := s.fd.WriteToUDP(pkt, s.peerAddr)
	return nonce, err
}

func checkedUint16Length(label string, length int) (uint16, error) {
	if length < 0 || length > math.MaxUint16 {
		return 0, fmt.Errorf("%s length %d exceeds %d", label, length, uint16(math.MaxUint16))
	}
	return uint16(length), nil
}

func checkedByteLength(label string, length int) (byte, error) {
	if length < 0 || length > math.MaxUint8 {
		return 0, fmt.Errorf("%s length %d exceeds %d", label, length, byte(math.MaxUint8))
	}
	return byte(length), nil
}

// SendRaw sends an already encoded discv5 packet to the peer.
func (s *Discv5Conn) SendRaw(pkt []byte) error {
	session.ObserveWrite(s.ctx, session.Write{Protocol: "discv5", Payload: pkt, Raw: true})
	_, err := s.fd.WriteToUDP(pkt, s.peerAddr)
	return err
}
