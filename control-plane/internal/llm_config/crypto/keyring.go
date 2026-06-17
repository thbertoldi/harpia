// Package crypto implements the two-layer envelope encryption used to store
// tenant LLM provider API keys at rest.
//
// Encryption model (design §3):
//
//  1. Each row gets a fresh random 32-byte DEK.
//  2. The provider API key is sealed with the DEK using AES-256-GCM and a
//     random 12-byte nonce. Wire layout: nonce || ciphertext || tag.
//  3. The DEK is wrapped with the active KEK version using AES-256-GCM and an
//     independent random 12-byte nonce. Wire layout: nonce || ciphertext || tag.
//  4. The row stores `kek_version`, `encrypted_dek`, and `encrypted_api_key`.
//
// Decryption picks the KEK referenced by the row's `kek_version`, unwraps the
// DEK, then opens the API-key ciphertext. KEKs are immutable per version;
// rotation introduces a new active version and never re-encrypts old rows.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
)

const (
	dekSize   = 32 // AES-256 key length.
	nonceSize = 12 // AES-GCM nonce length.
)

// ErrUnknownKEKVersion is returned when decryption references a KEK version
// the keyring does not have loaded. This typically means the operator
// removed a KEK version that still has live rows; rotation procedure
// (design §3) forbids this without a re-encryption migration.
var ErrUnknownKEKVersion = errors.New("unknown KEK version")

// ErrEmptyKeyring is returned when the keyring is constructed with no keys.
var ErrEmptyKeyring = errors.New("KEK keyring is empty")

// ErrUnknownActiveVersion is returned when the requested active version is
// not present in the keyring.
var ErrUnknownActiveVersion = errors.New("active KEK version not in keyring")

// KEKMaterial holds an unwrapped Key-Encryption Key.
type KEKMaterial struct {
	// Version is the operator-controlled label (e.g. "v1", "v2").
	Version string
	// Key is the raw 32-byte AES-256 KEK material.
	Key []byte
}

// Keyring is an immutable in-memory view of all KEK versions known to the
// process plus the single active version used for new writes.
//
// Construct one at startup from the mounted k8s secret and treat it as
// read-only thereafter. To rotate, restart the pod with a new keyring.
type Keyring struct {
	versions      map[string][]byte
	activeVersion string
}

// NewKeyring builds a keyring from the provided KEK material and pins
// activeVersion as the writer version. Each KEK must be exactly 32 bytes.
func NewKeyring(materials []KEKMaterial, activeVersion string) (*Keyring, error) {
	if len(materials) == 0 {
		return nil, ErrEmptyKeyring
	}
	versions := make(map[string][]byte, len(materials))
	for _, m := range materials {
		if m.Version == "" {
			return nil, errors.New("KEK version must not be empty")
		}
		if len(m.Key) != dekSize {
			return nil, fmt.Errorf("KEK %q must be %d bytes, got %d", m.Version, dekSize, len(m.Key))
		}
		if _, exists := versions[m.Version]; exists {
			return nil, fmt.Errorf("duplicate KEK version %q", m.Version)
		}
		buf := make([]byte, dekSize)
		copy(buf, m.Key)
		versions[m.Version] = buf
	}
	if _, ok := versions[activeVersion]; !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownActiveVersion, activeVersion)
	}
	return &Keyring{versions: versions, activeVersion: activeVersion}, nil
}

// ActiveVersion returns the writer KEK version label.
func (k *Keyring) ActiveVersion() string {
	return k.activeVersion
}

// HasVersion reports whether the keyring has a KEK with the given label.
func (k *Keyring) HasVersion(version string) bool {
	_, ok := k.versions[version]
	return ok
}

// kek returns the raw AES-256 KEK material for `version`.
func (k *Keyring) kek(version string) ([]byte, error) {
	raw, ok := k.versions[version]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownKEKVersion, version)
	}
	return raw, nil
}

// EncryptedRecord is the serializable output of envelope encryption.
type EncryptedRecord struct {
	KEKVersion       string
	EncryptedDEK     []byte // nonce || ciphertext || tag
	EncryptedPayload []byte // nonce || ciphertext || tag
}

// EncryptWithKeyring envelope-encrypts plaintext under the keyring's active
// KEK version. Caller's plaintext is fully consumed: no other copies are
// retained inside this package.
func EncryptWithKeyring(k *Keyring, plaintext []byte) (EncryptedRecord, error) {
	kek, err := k.kek(k.activeVersion)
	if err != nil {
		return EncryptedRecord{}, err
	}
	return encryptInternal(k.activeVersion, kek, plaintext)
}

// DecryptWithKeyring opens an envelope record by selecting the KEK that
// wrapped the record's DEK at write time.
func DecryptWithKeyring(k *Keyring, rec EncryptedRecord) ([]byte, error) {
	kek, err := k.kek(rec.KEKVersion)
	if err != nil {
		return nil, err
	}
	dek, err := openAESGCM(kek, rec.EncryptedDEK)
	if err != nil {
		return nil, fmt.Errorf("unwrap DEK (kek_version=%s): %w", rec.KEKVersion, err)
	}
	defer wipe(dek)
	plaintext, err := openAESGCM(dek, rec.EncryptedPayload)
	if err != nil {
		return nil, fmt.Errorf("open API key ciphertext: %w", err)
	}
	return plaintext, nil
}

func encryptInternal(version string, kek, plaintext []byte) (EncryptedRecord, error) {
	dek := make([]byte, dekSize)
	if _, err := rand.Read(dek); err != nil {
		return EncryptedRecord{}, fmt.Errorf("generate DEK: %w", err)
	}
	defer wipe(dek)

	encryptedPayload, err := sealAESGCM(dek, plaintext)
	if err != nil {
		return EncryptedRecord{}, fmt.Errorf("seal API key: %w", err)
	}

	encryptedDEK, err := sealAESGCM(kek, dek)
	if err != nil {
		return EncryptedRecord{}, fmt.Errorf("wrap DEK: %w", err)
	}

	return EncryptedRecord{
		KEKVersion:       version,
		EncryptedDEK:     encryptedDEK,
		EncryptedPayload: encryptedPayload,
	}, nil
}

func sealAESGCM(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("init AES: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("init GCM: %w", err)
	}
	nonce := make([]byte, nonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}
	ct := gcm.Seal(nil, nonce, plaintext, nil)
	out := make([]byte, 0, nonceSize+len(ct))
	out = append(out, nonce...)
	out = append(out, ct...)
	return out, nil
}

func openAESGCM(key, sealed []byte) ([]byte, error) {
	if len(sealed) < nonceSize {
		return nil, errors.New("ciphertext is shorter than nonce")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("init AES: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("init GCM: %w", err)
	}
	nonce := sealed[:nonceSize]
	ct := sealed[nonceSize:]
	pt, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("authenticate ciphertext: %w", err)
	}
	return pt, nil
}

func wipe(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
