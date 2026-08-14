# ADR-0006: AES-256-GCM Envelope Encryption for Secrets

## Status
Accepted

## Context
Services need runtime environment variables. Many of these are sensitive (API keys, DB passwords). They must be stored at rest without being readable in plaintext from the database file.

## Decision
Use **AES-256-GCM envelope encryption**:

1. A single **master key** (256-bit random) is loaded from the host environment (`MASTER_KEY_HEX`), never stored in the database.
2. Each secret has a unique **per-secret data key** (256-bit random) generated at write time.
3. The data key is XOR-encrypted with a 256-bit subkey derived from the master key using **HKDF-SHA256** with the secret ID as additional authenticated data (AAD).
4. Ciphertext stored in the `encrypted_secrets` table as `Envelope{ciphertext, nonce, key_version, aad}`.

**Schema column: `envelope_json`** - all 4 fields serialized as JSON.

## Consequences

**Positive:**
- Master key rotation possible: re-encrypt all envelopes with new key version
- No plaintext secrets ever written to SQLite WAL or backup
- AAD prevents cross-secret ciphertext swapping attacks
- Standard library only (`crypto/aes`, `crypto/cipher`, `golang.org/x/crypto/hkdf`)

**Negative:**
- Losing the master key = losing all secrets (mitigated by key backup instructions in SECURITY.md)
- Not HSM-backed (acceptable trade-off for self-hosted target)

## Key rotation protocol
1. Operator sets `MASTER_KEY_HEX_V2` and `CURRENT_KEY_VERSION=2`
2. API reads old key with version 1, re-encrypts with version 2 on next access
3. After all secrets are migrated, retire version 1

## Alternatives Considered
- **HashiCorp Vault** - too complex for self-hosted single-host
- **age encryption** - no standard key rotation mechanism
- **Plaintext** - rejected, violates security requirements
