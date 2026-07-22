package discovery

import (
	crand "crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/p2p/enode"

	"github.com/nethoxa-labs/raidan-sdk/session"
)

// Discv5Session contains the keys and source identity for an established discv5
// initiator session.
type Discv5Session struct {
	initiatorKey []byte
	recipientKey []byte
	sourceID     enode.ID
}

// Discv5OrdinaryPacket is a decrypted ordinary packet.
type Discv5OrdinaryPacket struct {
	MessageType byte
	Body        []byte
	SourceID    enode.ID
}

// EstablishSession performs the random-packet challenge and handshake needed
// before ordinary discv5 messages can be exchanged.
func (s *Discv5Conn) EstablishSession() (*Discv5Session, error) {
	nonce, err := s.SendRandomOrdinary()
	if err != nil {
		return nil, fmt.Errorf("send ordinary: %w", err)
	}
	w, err := s.readWhoAreYou(2*time.Second, &nonce)
	if err != nil {
		return nil, fmt.Errorf("read WHOAREYOU: %w", err)
	}
	return s.completeHandshake(w)
}

func (s *Discv5Conn) completeHandshake(w *WhoAreYou) (*Discv5Session, error) {
	_, established, err := s.sendHandshakeMessage(w)
	return established, err
}

// SendOrdinary encrypts and writes an ordinary packet with a message code.
func (s *Discv5Conn) SendOrdinary(sess *Discv5Session, msgType byte, body []byte) error {
	if sess == nil {
		return errors.New("nil session")
	}
	session.ObserveWrite(s.ctx, session.Write{Protocol: "discv5", Code: uint64(msgType), Payload: body})
	plain := make([]byte, 0, 1+len(body))
	plain = append(plain, msgType)
	plain = append(plain, body...)
	return s.sendOrdinaryPlainWithKey(sess, sess.initiatorKey, plain)
}

// SendOrdinaryPlain sends caller-supplied ordinary plaintext.
func (s *Discv5Conn) SendOrdinaryPlain(sess *Discv5Session, plain []byte) error {
	if sess == nil {
		return errors.New("nil session")
	}
	session.ObserveWrite(s.ctx, session.Write{Protocol: "discv5", Payload: plain, Raw: true})
	return s.sendOrdinaryPlainWithKey(sess, sess.initiatorKey, plain)
}

func (s *Discv5Conn) sendOrdinaryPlainWithKey(sess *Discv5Session, key, plain []byte) error {
	if sess == nil {
		return errors.New("nil session")
	}
	authdata := sess.sourceID[:]

	var nonce [12]byte
	if _, err := crand.Read(nonce[:]); err != nil {
		return err
	}
	staticHeader := make([]byte, discv5StaticHeaderSize)
	copy(staticHeader[0:6], discv5DefaultProtoID[:])
	binary.BigEndian.PutUint16(staticHeader[6:8], Discv5Version)
	staticHeader[8] = Discv5FlagOrdinary
	copy(staticHeader[9:21], nonce[:])
	binary.BigEndian.PutUint16(staticHeader[21:23], uint16(len(authdata)))

	var iv [16]byte
	if _, err := crand.Read(iv[:]); err != nil {
		return err
	}
	headerPlain := append(staticHeader, authdata...)
	headerMasked := MaskDiscv5Header(s.peerNodeID, iv, headerPlain)
	aad := append(append([]byte{}, iv[:]...), headerPlain...)
	ciphertext, err := SealDiscv5Message(key, nonce[:], plain, aad)
	if err != nil {
		return fmt.Errorf("gcm: %w", err)
	}
	packet := make([]byte, 0, len(iv)+len(headerMasked)+len(ciphertext))
	packet = append(packet, iv[:]...)
	packet = append(packet, headerMasked...)
	packet = append(packet, ciphertext...)
	_, err = s.fd.WriteToUDP(packet, s.peerAddr)
	return err
}

// ReadOrdinary reads and decrypts the next valid ordinary packet.
func (s *Discv5Conn) ReadOrdinary(sess *Discv5Session, timeout time.Duration) (*Discv5OrdinaryPacket, error) {
	if sess == nil {
		return nil, errors.New("nil session")
	}
	deadline := time.Now().Add(session.Timeout(s.ctx, timeout))
	buf := make([]byte, discv4MaxPacketSize)
	var lastErr error
	for {
		remain := time.Until(deadline)
		if remain <= 0 {
			if lastErr != nil {
				return nil, lastErr
			}
			return nil, errors.New("timeout")
		}
		n, from, err := readUDPWithContext(s.ctx, s.fd, buf, remain)
		if err != nil {
			return nil, err
		}
		if !SameUDPAddr(from, s.peerAddr) {
			continue
		}
		packet, err := s.decodeOrdinary(sess, buf[:n])
		if err == nil {
			return packet, nil
		}
		lastErr = err
	}
}

// ReadOrdinaryMessage waits for a decrypted ordinary packet with msgType.
func (s *Discv5Conn) ReadOrdinaryMessage(sess *Discv5Session, msgType byte, timeout time.Duration) (*Discv5OrdinaryPacket, error) {
	deadline := time.Now().Add(session.Timeout(s.ctx, timeout))
	for {
		remain := time.Until(deadline)
		if remain <= 0 {
			return nil, errors.New("timeout")
		}
		packet, err := s.ReadOrdinary(sess, remain)
		if err != nil {
			return nil, err
		}
		if packet.MessageType == msgType {
			return packet, nil
		}
	}
}

func (s *Discv5Conn) decodeOrdinary(sess *Discv5Session, packet []byte) (*Discv5OrdinaryPacket, error) {
	if len(packet) < discv5MinPacketSize {
		return nil, fmt.Errorf("packet too short: %d", len(packet))
	}
	var iv [16]byte
	copy(iv[:], packet[:16])
	staticMasked := packet[16 : 16+discv5StaticHeaderSize]
	staticHeader := make([]byte, len(staticMasked))
	applyMaskCTR(s.ourNodeID, iv, staticHeader, staticMasked)
	if string(staticHeader[0:6]) != string(discv5DefaultProtoID[:]) {
		return nil, errors.New("unexpected protocol-id in reply")
	}
	if version := binary.BigEndian.Uint16(staticHeader[6:8]); version != Discv5Version {
		return nil, fmt.Errorf("unexpected discv5 version 0x%04x", version)
	}
	if staticHeader[8] != Discv5FlagOrdinary {
		return nil, fmt.Errorf("expected ordinary flag (0x00), got 0x%02x", staticHeader[8])
	}
	authSize := binary.BigEndian.Uint16(staticHeader[21:23])
	headerSize := discv5StaticHeaderSize + int(authSize)
	if len(packet) < 16+headerSize+16 {
		return nil, fmt.Errorf("ordinary packet too short for authdata=%d", authSize)
	}
	headerMasked := packet[16 : 16+headerSize]
	headerPlain := make([]byte, len(headerMasked))
	applyMaskCTR(s.ourNodeID, iv, headerPlain, headerMasked)
	authdata := headerPlain[discv5StaticHeaderSize:]
	if len(authdata) != len(enode.ID{}) {
		return nil, fmt.Errorf("ordinary authdata size=%d, want 32", len(authdata))
	}
	var srcID enode.ID
	copy(srcID[:], authdata)
	if srcID != s.peerNodeID {
		return nil, errors.New("ordinary packet has unexpected source node ID")
	}
	aad := append(append([]byte{}, iv[:]...), headerPlain...)
	plain, err := gcmOpen(sess.recipientKey, headerPlain[9:21], packet[16+headerSize:], aad)
	if err != nil {
		return nil, fmt.Errorf("decrypt ordinary: %w", err)
	}
	if len(plain) == 0 {
		return nil, errors.New("ordinary plaintext has no msg type")
	}
	return &Discv5OrdinaryPacket{
		MessageType: plain[0],
		Body:        append([]byte(nil), plain[1:]...),
		SourceID:    srcID,
	}, nil
}
