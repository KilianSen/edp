package secret

import (
	"strings"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	box, err := Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, plain := range []string{"", "ghp_secrettoken", "with spaces & symbols=/+", strings.Repeat("x", 4096)} {
		enc, err := box.Encrypt(plain)
		if err != nil {
			t.Fatalf("encrypt %q: %v", plain, err)
		}
		if plain != "" && !strings.HasPrefix(enc, prefix) {
			t.Errorf("expected prefix on ciphertext for %q, got %q", plain, enc)
		}
		if plain != "" && strings.Contains(enc, plain) {
			t.Errorf("ciphertext leaks plaintext for %q", plain)
		}
		got, err := box.Decrypt(enc)
		if err != nil {
			t.Fatalf("decrypt: %v", err)
		}
		if got != plain {
			t.Errorf("round trip mismatch: got %q want %q", got, plain)
		}
	}
}

func TestEncryptIdempotent(t *testing.T) {
	box, _ := Load(t.TempDir())
	once, _ := box.Encrypt("token")
	twice, _ := box.Encrypt(once)
	if once != twice {
		t.Errorf("encrypting ciphertext again changed it: %q vs %q", once, twice)
	}
}

func TestLegacyPlaintextPassthrough(t *testing.T) {
	box, _ := Load(t.TempDir())
	// values without the version prefix are treated as legacy plaintext
	got, err := box.Decrypt("legacy-plain-value")
	if err != nil {
		t.Fatal(err)
	}
	if got != "legacy-plain-value" {
		t.Errorf("got %q", got)
	}
}

func TestTamperDetected(t *testing.T) {
	box, _ := Load(t.TempDir())
	enc, _ := box.Encrypt("token")
	// flip a character in the base64 body
	tampered := enc[:len(enc)-1] + "A"
	if tampered == enc {
		tampered = enc[:len(enc)-1] + "B"
	}
	if _, err := box.Decrypt(tampered); err == nil {
		t.Error("expected decryption of tampered ciphertext to fail")
	}
}

func TestKeyPersists(t *testing.T) {
	dir := t.TempDir()
	b1, _ := Load(dir)
	enc, _ := b1.Encrypt("token")
	b2, err := Load(dir) // reload uses the same key file
	if err != nil {
		t.Fatal(err)
	}
	got, err := b2.Decrypt(enc)
	if err != nil || got != "token" {
		t.Errorf("reloaded box could not decrypt: got %q err %v", got, err)
	}
}
