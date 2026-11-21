// Package utils provides deterministic identity and presentation helpers shared
// by the wire packages.
//
// DeterministicKey derives a stable secp256k1 identity from an explicit label.
// HumanBytes formats byte counts, and the terminal style constants can be used
// with scoped session output. Payload construction remains caller-owned.
//
//	key, err := utils.DeterministicKey("example/peer-1")
//	if err != nil {
//		return err
//	}
package utils
