# Security

## Reporting

Open a private advisory or email the maintainer if you find a vulnerability. Do not file a public issue for exploitable server/client bugs until a fix is ready.

## Threat model

See the short threat model in [README.md](README.md). Summary: peer traffic is encrypted end-to-end with HKDF-derived keys; the signaling/relay operator can still see metadata and MITM key exchange if you do not trust them. Prefer a self-hosted server among friends and `wss://` in front of the WebSocket.

## Secrets in git history

Older commits in the pre-OSS local history contained deploy tooling (host, SSH key material, key passphrase). That history must not be pushed publicly. The published `master` is a clean tree without those blobs. If you ever cloned an unclean history, rotate affected VPS SSH keys and credentials immediately.
