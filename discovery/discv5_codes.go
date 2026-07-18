package discovery

// Discv5 protocol identity, version, and packet flags.
const (
	Discv5ProtocolID    = "discv5"
	Discv5Version       = 0x0001
	Discv5FlagOrdinary  = 0x00 // ordinary
	Discv5FlagWhoAreYou = 0x01 // WHOAREYOU
	Discv5FlagHandshake = 0x02 // handshake
)

// Discv5 message codes.
const (
	Discv5MsgPing            = 0x01
	Discv5MsgPong            = 0x02
	Discv5MsgFindNode        = 0x03
	Discv5MsgNodes           = 0x04
	Discv5MsgTalkRequest     = 0x05
	Discv5MsgTalkResponse    = 0x06
	Discv5MsgRegTopic        = 0x07
	Discv5MsgTicket          = 0x08
	Discv5MsgRegConfirmation = 0x09
	Discv5MsgTopicQuery      = 0x0a
)

// Discv5AESKeySize is the protocol's AES-128 session-key size in bytes.
const Discv5AESKeySize = 16

const (
	discv5MaskingIVSize           = 16
	discv5StaticHeaderSize        = 23 // protocol-id(6) + version(2) + flag(1) + nonce(12) + auth-size(2)
	discv5GCMNonceSize            = 12
	discv5HandshakeAuthHeaderSize = 34 // src-id(32) + sig-size(1) + pubkey-size(1)
	discv5MinPacketSize           = 63
)

var discv5DefaultProtoID = [6]byte{'d', 'i', 's', 'c', 'v', '5'}
