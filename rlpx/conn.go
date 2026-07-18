package rlpx

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"io"
	"net"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/ecies"
	"github.com/ethereum/go-ethereum/crypto/keccak"
	"github.com/ethereum/go-ethereum/p2p/enode"
	rlpx "github.com/ethereum/go-ethereum/p2p/rlpx"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/golang/snappy"

	"github.com/nethoxa-labs/raidan-sdk/session"
)

const maxFrameSize = 0xffffff

// Conn is an authenticated RLPx frame connection.
type Conn struct {
	reader *rlpx.Conn
	net    net.Conn
	writer frameWriter

	mu     sync.Mutex
	snappy bool
}

type frameWriter struct {
	enc cipher.Stream
	mac frameMAC
}

type frameMAC struct {
	cipher cipher.Block
	hash   hash.Hash
	block  [16]byte
	sum    [32]byte
	seed   [32]byte
}

// DialRLPx opens a bare RLPx connection without sending Hello. It generates a
// fresh initiator key and returns both the framed and underlying connections.
func DialRLPx(ctx context.Context, target string) (*Conn, net.Conn, error) {
	key, err := crypto.GenerateKey()
	if err != nil {
		return nil, nil, fmt.Errorf("generate key: %w", err)
	}
	return DialWithKey(ctx, target, key)
}

// dial performs TCP and RLPx authentication and leaves the handshake deadline
// active so a caller can include any immediate protocol setup in the bound.
func dial(ctx context.Context, target string, key *ecdsa.PrivateKey) (*Conn, net.Conn, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if key == nil {
		return nil, nil, errors.New("nil initiator key")
	}
	node, err := enode.ParseV4(target)
	if err != nil {
		return nil, nil, fmt.Errorf("parse enode: %w", err)
	}
	addr := net.JoinHostPort(node.IP().String(), fmt.Sprintf("%d", node.TCP()))
	dialCtx, cancel := context.WithTimeout(ctx, session.Timeout(ctx, 5*time.Second))
	defer cancel()
	fd, err := (&net.Dialer{}).DialContext(dialCtx, "tcp", addr)
	if err != nil {
		return nil, nil, fmt.Errorf("dial: %w", err)
	}
	if err := fd.SetDeadline(time.Now().Add(session.Timeout(ctx, 10*time.Second))); err != nil {
		_ = fd.Close()
		return nil, nil, fmt.Errorf("set handshake deadline: %w", err)
	}
	secrets, err := initiatorHandshake(fd, key, node.Pubkey())
	if err != nil {
		_ = fd.Close()
		return nil, nil, fmt.Errorf("rlpx handshake: %w", err)
	}
	rc, err := newConn(fd, secrets)
	if err != nil {
		_ = fd.Close()
		return nil, nil, err
	}
	return rc, fd, nil
}

// DialWithKey performs an initiator handshake with an explicit local identity.
// The returned connection has no socket deadline.
func DialWithKey(ctx context.Context, target string, key *ecdsa.PrivateKey) (*Conn, net.Conn, error) {
	conn, network, err := dial(ctx, target, key)
	if err != nil {
		return nil, nil, err
	}
	if err := clearDeadline(network); err != nil {
		_ = network.Close()
		return nil, nil, err
	}
	return conn, network, nil
}

func clearDeadline(fd net.Conn) error {
	if err := fd.SetDeadline(time.Time{}); err != nil {
		return fmt.Errorf("clear handshake deadline: %w", err)
	}
	return nil
}

// WaitClose reports whether the peer closed or sent Disconnect. A deadline
// expiry means the connection remained open.
func WaitClose(ctx context.Context, rc *Conn, fd net.Conn, timeout time.Duration) bool {
	closed, _ := WaitCloseReason(ctx, rc, fd, timeout)
	return closed
}

// WaitCloseReason is WaitClose plus the peer's decoded Disconnect reason.
func WaitCloseReason(ctx context.Context, rc *Conn, fd net.Conn, timeout time.Duration) (bool, string) {
	return waitCloseReason(ctx, rc, fd, timeout)
}

type messageReader interface {
	Read() (uint64, []byte, int, error)
}

type readDeadlineSetter interface {
	SetReadDeadline(time.Time) error
}

func waitCloseReason(ctx context.Context, rc messageReader, fd readDeadlineSetter, timeout time.Duration) (bool, string) {
	if err := fd.SetReadDeadline(time.Now().Add(session.Timeout(ctx, timeout))); err != nil {
		return false, ""
	}
	defer func() { _ = fd.SetReadDeadline(time.Time{}) }()
	for {
		code, data, _, err := rc.Read()
		if err != nil {
			if isTimeoutError(err) {
				return false, ""
			}
			return true, ""
		}
		if code == DiscMsg {
			_, reason := DecodeDisconnectReason(data)
			return true, reason
		}
	}
}

func isTimeoutError(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func newConn(network net.Conn, secrets rlpx.Secrets) (*Conn, error) {
	block, err := aes.NewCipher(secrets.AES)
	if err != nil {
		return nil, fmt.Errorf("create frame cipher: %w", err)
	}
	macBlock, err := aes.NewCipher(secrets.MAC)
	if err != nil {
		return nil, fmt.Errorf("create frame MAC cipher: %w", err)
	}
	reader := rlpx.NewConn(network, nil)
	reader.InitWithSecrets(secrets)
	return &Conn{
		reader: reader,
		net:    network,
		writer: frameWriter{
			enc: cipher.NewCTR(block, make([]byte, block.BlockSize())),
			mac: frameMAC{cipher: macBlock, hash: secrets.EgressMAC},
		},
	}, nil
}

func (c *Conn) Read() (uint64, []byte, int, error) { return c.reader.Read() }

func (c *Conn) Write(code uint64, payload []byte) (uint32, error) {
	if c == nil {
		return 0, errors.New("rlpx connection is nil")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(payload) > maxFrameSize {
		return 0, fmt.Errorf("frame payload too large: %d bytes", len(payload))
	}
	data := payload
	if c.snappy {
		data = snappy.Encode(nil, payload)
	}
	plain := rlp.AppendUint64(nil, code)
	plain = append(plain, data...)
	if err := c.writer.write(c.net, []byte{0xc2, 0x80, 0x80}, plain); err != nil {
		return 0, err
	}
	return uint32(len(data)), nil
}

// SetSnappy enables or disables devp2p payload compression.
func (c *Conn) SetSnappy(enabled bool) {
	c.mu.Lock()
	c.snappy = enabled
	c.mu.Unlock()
	c.reader.SetSnappy(enabled)
}

// SetReadDeadline sets the underlying socket read deadline.
func (c *Conn) SetReadDeadline(deadline time.Time) error { return c.net.SetReadDeadline(deadline) }

// SetWriteDeadline sets the underlying socket write deadline.
func (c *Conn) SetWriteDeadline(deadline time.Time) error { return c.net.SetWriteDeadline(deadline) }

// SetDeadline sets both underlying socket deadlines.
func (c *Conn) SetDeadline(deadline time.Time) error { return c.net.SetDeadline(deadline) }

// Close closes the underlying socket.
func (c *Conn) Close() error { return c.net.Close() }

// WriteRawFrame writes one authenticated RLPx frame from caller-owned header
// data and plaintext. plaintext must already contain any message-code prefix.
func (c *Conn) WriteRawFrame(headerData, plaintext []byte) error {
	if c == nil {
		return errors.New("rlpx connection is nil")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.writer.write(c.net, headerData, plaintext)
}

func (w *frameWriter) write(dst io.Writer, headerData, plaintext []byte) error {
	if len(headerData) > 13 {
		return fmt.Errorf("header-data too large: %d bytes", len(headerData))
	}
	if len(plaintext) > maxFrameSize {
		return fmt.Errorf("frame payload too large: %d bytes", len(plaintext))
	}
	header := make([]byte, 16)
	putUint24(header, uint32(len(plaintext)))
	copy(header[3:], headerData)
	w.enc.XORKeyStream(header, header)

	frame := append([]byte(nil), plaintext...)
	if padding := len(frame) % aes.BlockSize; padding > 0 {
		frame = append(frame, make([]byte, aes.BlockSize-padding)...)
	}
	w.enc.XORKeyStream(frame, frame)

	out := make([]byte, 0, 32+len(frame)+16)
	out = append(out, header...)
	out = append(out, w.mac.header(header)...)
	out = append(out, frame...)
	out = append(out, w.mac.frame(frame)...)
	for len(out) != 0 {
		n, err := dst.Write(out)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		out = out[n:]
	}
	return nil
}

func (m *frameMAC) header(header []byte) []byte {
	return m.compute(m.hash.Sum(m.sum[:0]), header)
}

func (m *frameMAC) frame(frame []byte) []byte {
	_, _ = m.hash.Write(frame)
	seed := m.hash.Sum(m.seed[:0])
	return m.compute(seed, seed[:16])
}

func (m *frameMAC) compute(sum, seed []byte) []byte {
	m.cipher.Encrypt(m.block[:], sum)
	for i := range m.block {
		m.block[i] ^= seed[i]
	}
	_, _ = m.hash.Write(m.block[:])
	return append([]byte(nil), m.hash.Sum(m.sum[:0])[:16]...)
}

type authResponse struct {
	RandomPubkey [64]byte
	Nonce        [32]byte
	Version      uint
	Rest         []rlp.RawValue `rlp:"tail"`
}

func initiatorHandshake(network io.ReadWriter, local *ecdsa.PrivateKey, remote *ecdsa.PublicKey) (rlpx.Secrets, error) {
	if local == nil || remote == nil {
		return rlpx.Secrets{}, errors.New("RLPx identity keys are required")
	}
	eph, err := ecies.GenerateKey(rand.Reader, crypto.S256(), nil)
	if err != nil {
		return rlpx.Secrets{}, err
	}
	initNonce := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, initNonce); err != nil {
		return rlpx.Secrets{}, err
	}
	staticSecret, err := ecies.ImportECDSA(local).GenerateShared(ecies.ImportECDSAPublic(remote), 16, 16)
	if err != nil {
		return rlpx.Secrets{}, err
	}
	signature, err := crypto.Sign(xorBytesAuth(staticSecret, initNonce), eph.ExportECDSA())
	if err != nil {
		return rlpx.Secrets{}, err
	}
	message := authMsgV4{Version: 4}
	copy(message.Signature[:], signature)
	copy(message.InitiatorPubkey[:], crypto.FromECDSAPub(&local.PublicKey)[1:])
	copy(message.Nonce[:], initNonce)
	authPacket, err := sealHandshakeMessage(&message, ecies.ImportECDSAPublic(remote))
	if err != nil {
		return rlpx.Secrets{}, err
	}
	if err := writeAll(network, authPacket); err != nil {
		return rlpx.Secrets{}, err
	}
	var response authResponse
	responsePacket, err := readHandshakeMessage(network, local, &response)
	if err != nil {
		return rlpx.Secrets{}, err
	}
	remoteEph, err := crypto.UnmarshalPubkey(append([]byte{4}, response.RandomPubkey[:]...))
	if err != nil {
		return rlpx.Secrets{}, fmt.Errorf("decode responder ephemeral key: %w", err)
	}
	ecdheSecret, err := eph.GenerateShared(ecies.ImportECDSAPublic(remoteEph), 16, 16)
	if err != nil {
		return rlpx.Secrets{}, err
	}
	shared := crypto.Keccak256(ecdheSecret, crypto.Keccak256(response.Nonce[:], initNonce))
	aesSecret := crypto.Keccak256(ecdheSecret, shared)
	macSecret := crypto.Keccak256(ecdheSecret, aesSecret)
	egress := keccak.NewLegacyKeccak256()
	_, _ = egress.Write(xorBytesAuth(macSecret, response.Nonce[:]))
	_, _ = egress.Write(authPacket)
	ingress := keccak.NewLegacyKeccak256()
	_, _ = ingress.Write(xorBytesAuth(macSecret, initNonce))
	_, _ = ingress.Write(responsePacket)
	return rlpx.Secrets{AES: aesSecret, MAC: macSecret, EgressMAC: egress, IngressMAC: ingress}, nil
}

func sealHandshakeMessage(message any, remote *ecies.PublicKey) ([]byte, error) {
	plain, err := rlp.EncodeToBytes(message)
	if err != nil {
		return nil, err
	}
	padding := make([]byte, 100)
	if _, err := io.ReadFull(rand.Reader, padding); err != nil {
		return nil, err
	}
	plain = append(plain, padding...)
	size := len(plain) + rlpxAuthEciesOverhead
	if size > 0xffff {
		return nil, errors.New("handshake packet too large")
	}
	prefix := []byte{byte(size >> 8), byte(size)}
	encrypted, err := ecies.Encrypt(rand.Reader, remote, plain, nil, prefix)
	if err != nil {
		return nil, err
	}
	return append(prefix, encrypted...), nil
}

func readHandshakeMessage(src io.Reader, local *ecdsa.PrivateKey, output any) ([]byte, error) {
	prefix := make([]byte, 2)
	if _, err := io.ReadFull(src, prefix); err != nil {
		return nil, err
	}
	size := binary.BigEndian.Uint16(prefix)
	if size > 2048 {
		return nil, fmt.Errorf("handshake packet too large: %d", size)
	}
	packet := make([]byte, size)
	if _, err := io.ReadFull(src, packet); err != nil {
		return nil, err
	}
	plain, err := ecies.ImportECDSA(local).Decrypt(packet, nil, prefix)
	if err != nil {
		return nil, err
	}
	if err := rlp.NewStream(bytes.NewReader(plain), 0).Decode(output); err != nil {
		return nil, err
	}
	return append(prefix, packet...), nil
}

func writeAll(dst io.Writer, data []byte) error {
	for len(data) != 0 {
		n, err := dst.Write(data)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	return nil
}

func putUint24(output []byte, value uint32) {
	output[0] = byte(value >> 16)
	output[1] = byte(value >> 8)
	output[2] = byte(value)
}
