package discovery

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"testing"
	"time"
)

func TestKeyCipherBindsCiphertextToAgentAndKey(t *testing.T) {
	master := make([]byte, 32)
	if _, err := rand.Read(master); err != nil {
		t.Fatal(err)
	}
	cipherState, err := newKeyCipher(base64.StdEncoding.EncodeToString(master))
	if err != nil {
		t.Fatal(err)
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, nonce, err := cipherState.encrypt("agent-one", "key-one", private)
	if err != nil {
		t.Fatal(err)
	}
	key := SigningKey{ID: "key-one", AgentID: "agent-one", PublicKey: public, EncryptedPrivateKey: ciphertext, Nonce: nonce}
	decoded, err := cipherState.decrypt(key)
	if err != nil {
		t.Fatal(err)
	}
	if string(decoded) != string(private) {
		t.Fatal("decrypted private key changed")
	}
	key.AgentID = "agent-two"
	if _, err = cipherState.decrypt(key); err == nil {
		t.Fatal("ciphertext must not decrypt for another Agent")
	}
}

func TestSignedDocumentCarriesVerifiableEd25519Proof(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	key := SigningKey{ID: "key-one", PublicKey: public}
	document := Document{SchemaVersion: draftSchemaVersion, PublicationID: "publication-one", ProfileVersion: 1, Subject: map[string]any{"id": "agent-one"}, Services: []map[string]any{}, Claims: []Claim{}, Privacy: map[string]any{}, ValidFrom: time.Unix(1, 0).UTC(), ValidUntil: time.Unix(2, 0).UTC(), Revocation: map[string]any{}}
	signed, _, protected, signature, err := signDocument(document, key, private)
	if err != nil {
		t.Fatal(err)
	}
	if signed.Proof["digest"] == "" {
		t.Fatal("missing digest")
	}
	if protected == "" || signature == "" {
		t.Fatal("missing compact JWS parts")
	}
}

func TestNormalizeHostnameRejectsLocalAndAmbiguousHosts(t *testing.T) {
	for _, value := range []string{"localhost", "127.0.0.1", "example.com:443", "*.example.com", "singlelabel"} {
		if _, ok := normalizeHostname(value); ok {
			t.Fatalf("accepted %q", value)
		}
	}
	if got, ok := normalizeHostname("Agent.Example.COM."); !ok || got != "agent.example.com" {
		t.Fatalf("hostname = %q, %v", got, ok)
	}
}
