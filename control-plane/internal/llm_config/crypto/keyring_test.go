package crypto

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func mustKEK(t *testing.T) []byte {
	t.Helper()
	b := make([]byte, dekSize)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("read random: %v", err)
	}
	return b
}

func mustKeyring(t *testing.T, versions ...KEKMaterial) *Keyring {
	t.Helper()
	if len(versions) == 0 {
		versions = []KEKMaterial{{Version: "v1", Key: mustKEK(t)}}
	}
	k, err := NewKeyring(versions, versions[len(versions)-1].Version)
	if err != nil {
		t.Fatalf("new keyring: %v", err)
	}
	return k
}

func TestEnvelopeRoundTrip(t *testing.T) {
	t.Parallel()

	keyring := mustKeyring(t)
	plaintext := []byte("sk-ant-tenant-deadbeef-EXAMPLE-ONLY")

	rec, err := EncryptWithKeyring(keyring, plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if rec.KEKVersion != keyring.ActiveVersion() {
		t.Fatalf("KEKVersion = %q, want %q", rec.KEKVersion, keyring.ActiveVersion())
	}
	if bytes.Contains(rec.EncryptedPayload, plaintext) {
		t.Fatal("ciphertext leaked plaintext bytes")
	}
	if bytes.Contains(rec.EncryptedDEK, plaintext) {
		t.Fatal("wrapped DEK contained plaintext")
	}

	got, err := DecryptWithKeyring(keyring, rec)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("plaintext mismatch: got %q want %q", got, plaintext)
	}
}

func TestEnvelopeNonceUniqueness(t *testing.T) {
	t.Parallel()
	keyring := mustKeyring(t)
	plaintext := []byte("repeat-me")

	rec1, err := EncryptWithKeyring(keyring, plaintext)
	if err != nil {
		t.Fatalf("encrypt 1: %v", err)
	}
	rec2, err := EncryptWithKeyring(keyring, plaintext)
	if err != nil {
		t.Fatalf("encrypt 2: %v", err)
	}

	if bytes.Equal(rec1.EncryptedDEK, rec2.EncryptedDEK) {
		t.Fatal("wrapped DEKs are identical; nonce reuse")
	}
	if bytes.Equal(rec1.EncryptedPayload, rec2.EncryptedPayload) {
		t.Fatal("API key ciphertexts are identical; nonce reuse")
	}
}

func TestEnvelopeTamperDetection(t *testing.T) {
	t.Parallel()
	keyring := mustKeyring(t)
	rec, err := EncryptWithKeyring(keyring, []byte("hello"))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	tampered := EncryptedRecord{
		KEKVersion:       rec.KEKVersion,
		EncryptedDEK:     append([]byte{}, rec.EncryptedDEK...),
		EncryptedPayload: append([]byte{}, rec.EncryptedPayload...),
	}
	tampered.EncryptedPayload[len(tampered.EncryptedPayload)-1] ^= 0x01

	if _, err := DecryptWithKeyring(keyring, tampered); err == nil {
		t.Fatal("expected GCM authentication failure")
	}
}

func TestUnknownKEKVersionRejected(t *testing.T) {
	t.Parallel()
	keyring := mustKeyring(t)
	rec, _ := EncryptWithKeyring(keyring, []byte("payload"))
	rec.KEKVersion = "v9-does-not-exist"

	_, err := DecryptWithKeyring(keyring, rec)
	if !errors.Is(err, ErrUnknownKEKVersion) {
		t.Fatalf("err = %v, want ErrUnknownKEKVersion", err)
	}
}

func TestCrossVersionDecryptAfterRotation(t *testing.T) {
	t.Parallel()

	v1 := KEKMaterial{Version: "v1", Key: mustKEK(t)}
	v2 := KEKMaterial{Version: "v2", Key: mustKEK(t)}

	// Encrypt under v1, then rotate active to v2 and decrypt the old row.
	older, err := NewKeyring([]KEKMaterial{v1}, "v1")
	if err != nil {
		t.Fatalf("new keyring v1: %v", err)
	}
	plaintext := []byte("rotation-safe-payload")
	rec, err := EncryptWithKeyring(older, plaintext)
	if err != nil {
		t.Fatalf("encrypt v1: %v", err)
	}

	rotated, err := NewKeyring([]KEKMaterial{v1, v2}, "v2")
	if err != nil {
		t.Fatalf("new keyring v2: %v", err)
	}
	if got, _ := EncryptWithKeyring(rotated, plaintext); got.KEKVersion != "v2" {
		t.Fatalf("new writes should use active KEK v2, got %q", got.KEKVersion)
	}

	// Old row still decrypts because v1 is still in the keyring.
	if got, err := DecryptWithKeyring(rotated, rec); err != nil || !bytes.Equal(got, plaintext) {
		t.Fatalf("post-rotation decrypt of v1 row failed: got=%q err=%v", got, err)
	}
}

func TestKeyringRejectsBadInputs(t *testing.T) {
	t.Parallel()

	if _, err := NewKeyring(nil, "v1"); !errors.Is(err, ErrEmptyKeyring) {
		t.Fatalf("expected ErrEmptyKeyring, got %v", err)
	}
	if _, err := NewKeyring([]KEKMaterial{{Version: "", Key: mustKEK(t)}}, ""); err == nil {
		t.Fatal("expected error for empty version label")
	}
	if _, err := NewKeyring([]KEKMaterial{{Version: "v1", Key: []byte("too short")}}, "v1"); err == nil {
		t.Fatal("expected error for short KEK")
	}
	if _, err := NewKeyring([]KEKMaterial{{Version: "v1", Key: mustKEK(t)}}, "vX"); !errors.Is(err, ErrUnknownActiveVersion) {
		t.Fatalf("expected ErrUnknownActiveVersion, got %v", err)
	}
}

func TestLoadKeyringFromEnv(t *testing.T) {
	v1 := mustKEK(t)
	v2 := mustKEK(t)
	t.Setenv("HARPIA_LLM_KEK_V1_B64", base64.StdEncoding.EncodeToString(v1))
	t.Setenv("HARPIA_LLM_KEK_V2_B64", base64.StdEncoding.EncodeToString(v2))
	t.Setenv("HARPIA_LLM_KEK_ACTIVE", "v2")

	k, err := LoadKeyring(DefaultKeyringConfig())
	if err != nil {
		t.Fatalf("LoadKeyring: %v", err)
	}
	if k.ActiveVersion() != "v2" {
		t.Fatalf("active = %q, want v2", k.ActiveVersion())
	}
	if !k.HasVersion("v1") || !k.HasVersion("v2") {
		t.Fatalf("expected both v1 and v2 loaded")
	}
}

func TestLoadKeyringFromMount(t *testing.T) {
	dir := t.TempDir()
	v1 := mustKEK(t)
	v2 := mustKEK(t)
	if err := os.WriteFile(filepath.Join(dir, "v1"), v1, 0o600); err != nil {
		t.Fatalf("write v1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "v2"), []byte(base64.StdEncoding.EncodeToString(v2)), 0o600); err != nil {
		t.Fatalf("write v2: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "active"), []byte("v2"), 0o600); err != nil {
		t.Fatalf("write active: %v", err)
	}

	cfg := DefaultKeyringConfig()
	cfg.MountDir = dir
	k, err := LoadKeyring(cfg)
	if err != nil {
		t.Fatalf("LoadKeyring: %v", err)
	}
	if k.ActiveVersion() != "v2" {
		t.Fatalf("active = %q, want v2", k.ActiveVersion())
	}
	if !k.HasVersion("v1") || !k.HasVersion("v2") {
		t.Fatalf("expected both v1 and v2 loaded")
	}
}
