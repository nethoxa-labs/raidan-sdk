package discovery

// Discv5 handshake-message cryptographic helpers. These functions build ENRs
// and identity-signature inputs using the wire format expected by a recipient.

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	crand "crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"golang.org/x/crypto/hkdf"
)

// IDSignatureHash computes the discv5 id-signature input. Per
// https://github.com/ethereum/devp2p/blob/master/discv5/discv5-wire.md#authentication-headers
// the input is:
//
//	sha256("discovery v5 identity proof" || challenge_data || eph_pub_compressed || node_id_B)
func IDSignatureHash(challenge []byte, ephPub []byte, peerID enode.ID) []byte {
	h := sha256.New()
	h.Write([]byte("discovery v5 identity proof"))
	h.Write(challenge)
	h.Write(ephPub)
	h.Write(peerID[:])
	return h.Sum(nil)
}

// SignIDSignature produces a 64-byte (r||s) id-signature for the discv5
// handshake-message, signed with `priv`, binding the supplied ephemeral
// pubkey + challenge_data + peer node id.
func SignIDSignature(challenge []byte, ephPub []byte, peerID enode.ID, priv *ecdsa.PrivateKey) ([]byte, error) {
	if priv == nil {
		return nil, errors.New("sign discv5 identity: nil private key")
	}
	h := IDSignatureHash(challenge, ephPub, peerID)
	full, err := crypto.Sign(h, priv)
	if err != nil {
		return nil, err
	}
	return full[:64], nil
}

// DeriveDiscv5SessionKeys derives the initiator and recipient AES keys for a
// discv5 handshake from ECDH and the protocol HKDF transcript.
func DeriveDiscv5SessionKeys(ephemeral *ecdsa.PrivateKey, remoteStatic *ecdsa.PublicKey, challenge []byte, sourceID, recipientID enode.ID) ([]byte, []byte, error) {
	if ephemeral == nil || remoteStatic == nil {
		return nil, nil, errors.New("discv5 session key derivation requires both local and remote keys")
	}
	shared, err := ecdhSecret(ephemeral, remoteStatic)
	if err != nil {
		return nil, nil, err
	}
	info := append([]byte("discovery v5 key agreement"), sourceID[:]...)
	info = append(info, recipientID[:]...)
	kdf := hkdf.New(sha256.New, shared, challenge, info)
	initiatorKey := make([]byte, Discv5AESKeySize)
	if _, err := kdf.Read(initiatorKey); err != nil {
		return nil, nil, err
	}
	recipientKey := make([]byte, Discv5AESKeySize)
	if _, err := kdf.Read(recipientKey); err != nil {
		return nil, nil, err
	}
	return initiatorKey, recipientKey, nil
}

// SealDiscv5Message encrypts one discv5 message using AES-GCM and caller-owned
// nonce and associated data.
func SealDiscv5Message(key, nonce, plaintext, associatedData []byte) ([]byte, error) {
	return gcmSeal(key, nonce, plaintext, associatedData)
}

func (s *Discv5Conn) sendHandshakeMessage(w *WhoAreYou) ([12]byte, *Discv5Session, error) {
	if w == nil {
		return [12]byte{}, nil, errors.New("nil WHOAREYOU challenge")
	}
	ephPriv, err := crypto.GenerateKey()
	if err != nil {
		return [12]byte{}, nil, fmt.Errorf("eph keygen: %w", err)
	}
	ephPubCompressed := crypto.CompressPubkey(&ephPriv.PublicKey)
	computedPublicKeySize, err := checkedByteLength("ephemeral public key", len(ephPubCompressed))
	if err != nil {
		return [12]byte{}, nil, err
	}

	srcID := s.ourNodeID
	sig, err := SignIDSignature(w.ChallengeData, ephPubCompressed, s.peerNodeID, s.ourPriv)
	if err != nil {
		return [12]byte{}, nil, fmt.Errorf("sign: %w", err)
	}
	initKey, recvKey, err := DeriveDiscv5SessionKeys(ephPriv, s.peerStatic, w.ChallengeData, srcID, s.peerNodeID)
	if err != nil {
		return [12]byte{}, nil, fmt.Errorf("derive session keys: %w", err)
	}

	sigSize := byte(len(sig))
	pubkeySize := computedPublicKeySize
	record, err := BuildV4ENR(1, s.ourPriv)
	if err != nil {
		return [12]byte{}, nil, fmt.Errorf("build record: %w", err)
	}

	authdataLength := discv5HandshakeAuthHeaderSize + len(sig) + len(ephPubCompressed) + len(record)
	computedAuthSize, err := checkedUint16Length("discv5 handshake authdata", authdataLength)
	if err != nil {
		return [12]byte{}, nil, err
	}
	authdata := make([]byte, 0, authdataLength)
	authdata = append(authdata, srcID[:]...)
	authdata = append(authdata, sigSize)
	authdata = append(authdata, pubkeySize)
	authdata = append(authdata, sig...)
	authdata = append(authdata, ephPubCompressed...)
	authdata = append(authdata, record...)

	var nonce [12]byte
	if _, err := crand.Read(nonce[:]); err != nil {
		return nonce, nil, err
	}
	staticHeader := make([]byte, discv5StaticHeaderSize)
	copy(staticHeader[0:6], discv5DefaultProtoID[:])
	binary.BigEndian.PutUint16(staticHeader[6:8], Discv5Version)
	staticHeader[8] = Discv5FlagHandshake
	copy(staticHeader[9:21], nonce[:])
	binary.BigEndian.PutUint16(staticHeader[21:23], computedAuthSize)

	plain := []byte{Discv5MsgPing, 0xC2, 0x01, 0x80}

	var iv [16]byte
	if _, err := crand.Read(iv[:]); err != nil {
		return nonce, nil, err
	}
	headerPlain := append(staticHeader, authdata...)
	headerMasked := MaskDiscv5Header(s.peerNodeID, iv, headerPlain)

	aad := append(append([]byte{}, iv[:]...), headerPlain...)
	ct, err := SealDiscv5Message(initKey, nonce[:], plain, aad)
	if err != nil {
		return nonce, nil, fmt.Errorf("gcm: %w", err)
	}

	pkt := make([]byte, 0, len(iv)+len(headerMasked)+len(ct))
	pkt = append(pkt, iv[:]...)
	pkt = append(pkt, headerMasked...)
	pkt = append(pkt, ct...)
	_, err = s.fd.WriteToUDP(pkt, s.peerAddr)
	if err != nil {
		return nonce, nil, err
	}
	return nonce, &Discv5Session{initiatorKey: initKey, recipientKey: recvKey, sourceID: srcID}, nil
}

// ecdhSecret returns the 33-byte (compressed-prefix) shared secret
// priv·peerPub. Mirrors what v5wire.ecdh does internally before HKDF.
func ecdhSecret(priv *ecdsa.PrivateKey, peer *ecdsa.PublicKey) ([]byte, error) {
	peerKey, err := secp256k1.ParsePubKey(crypto.CompressPubkey(peer))
	if err != nil {
		return nil, fmt.Errorf("parse peer public key: %w", err)
	}
	privateKey := secp256k1.PrivKeyFromBytes(crypto.FromECDSA(priv))
	var peerPoint, sharedPoint secp256k1.JacobianPoint
	peerKey.AsJacobian(&peerPoint)
	secp256k1.ScalarMultNonConst(&privateKey.Key, &peerPoint, &sharedPoint)
	if sharedPoint.Z.IsZero() {
		return nil, errors.New("ecdh produced point at infinity")
	}
	sharedPoint.ToAffine()
	return secp256k1.NewPublicKey(&sharedPoint.X, &sharedPoint.Y).SerializeCompressed(), nil
}

// gcmSeal AES-128-GCM seals plaintext with associated data.
func gcmSeal(key, nonce, plaintext, aad []byte) ([]byte, error) {
	if len(nonce) != discv5GCMNonceSize {
		return nil, fmt.Errorf("discv5 nonce is %d bytes, want %d", len(nonce), discv5GCMNonceSize)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	g, err := cipher.NewGCMWithNonceSize(block, discv5GCMNonceSize)
	if err != nil {
		return nil, err
	}
	return g.Seal(nil, nonce, plaintext, aad), nil
}

func gcmOpen(key, nonce, ciphertext, aad []byte) ([]byte, error) {
	if len(nonce) != discv5GCMNonceSize {
		return nil, fmt.Errorf("discv5 nonce is %d bytes, want %d", len(nonce), discv5GCMNonceSize)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	g, err := cipher.NewGCMWithNonceSize(block, discv5GCMNonceSize)
	if err != nil {
		return nil, err
	}
	return g.Open(nil, nonce, ciphertext, aad)
}
