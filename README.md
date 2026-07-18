# raidan-sdk

Independent Go building blocks for Ethereum execution and consensus networking:
wire types, codecs, discovery, handshakes, reusable sessions, request matching,
and caller-controlled protocol primitives.

The initial API requires Go 1.26.5 or newer.

```bash
go get github.com/nethoxa-labs/raidan-sdk@v0.1.0
```

Packages are intentionally product-independent and can be imported separately.

## Packages

| Package | Description |
| --- | --- |
| module root | Portable `Test`, `Target`, `Result`, and case execution |
| `target` | Execution and consensus endpoints and local endpoint discovery |
| `session` | Context scope, deadline capping, output, metadata, and write observation |
| `result` | Structured operation verdicts and optional presentation |
| `reqresp` | Consensus protocol IDs, libp2p sessions, SSZ-snappy framing, and requests |
| `gossipsub` | Fork-digest topics, peer waiting, and caller-encoded gossip publication |
| `discovery` | ENR and enode codecs, DNS discovery, discv4, and discv5 |
| `rlpx` | RLPx authentication, encrypted framing, Hello, and disconnect handling |
| `eth` | ETH negotiation, caller-owned wire shapes, request matching, and raw wire access |
| `snap` | SNAP negotiation, messages, and request matching |
| `rpc` | Execution JSON-RPC and cached chain metadata |
| `wit` | WIT connections, messages, and caller-parameterized encoders |
| `utils` | Deterministic keys, byte formatting, and terminal styles |

## Portable cases

A case is a normal Go function and imports the SDK surface it needs:

```go
func Test(ctx context.Context, target sdk.Target) sdk.Result {
	ctx = session.With(ctx, session.Scope{Client: target.Client})
	return result.Accept("case completed")
}
```

The public SDK intentionally contains no private behavior catalog, outcome-specific
payload presets, client-specific thresholds, or verdict oracles. A private case
owns those decisions inside its `Test` function and uses SDK packages for
transport, protocol state, and result values. Upstream Ethereum primitives are
imported directly under their native package names. The
root package exposes only SDK-owned behavior such as `Run`, `EncodeRLPBytes`,
and `EncodeRLPList`; it does not re-export pass-through function wrappers.
Dependency-owned Ethereum primitives are imported directly from their upstream
packages.

An embedding program runs an imported private case without depending on a
Raidan catalog or worker package:

```go
outcome := sdk.Run(ctx, testcase.Test, target, sdk.Scope{
	Output: os.Stdout,
	Client: target.Client,
})
```

`sdk.Run` supplies output and client metadata. Callers can attach the neutral
`session.Observer` through `sdk.Scope` when they need to trace protocol writes.
Case registries, private source, candidate selection, and persistence do not
belong in the SDK.

## License

Apache-2.0. See [LICENSE](LICENSE).
