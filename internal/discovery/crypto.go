package discovery

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
)

type keyCipher struct {
	key []byte
}

func newKeyCipher(encoded string) (*keyCipher, error) {
	if encoded == "" {
		return &keyCipher{}, nil
	}
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(key) != 32 {
		return nil, errors.New("AGENT_KEY_ENCRYPTION_KEY must be base64 for exactly 32 bytes")
	}
	return &keyCipher{key: key}, nil
}

func (cipherState *keyCipher) available() bool { return len(cipherState.key) == 32 }

func (cipherState *keyCipher) encrypt(agentID, keyID string, privateKey ed25519.PrivateKey) ([]byte, []byte, error) {
	if !cipherState.available() {
		return nil, nil, ErrUnavailable
	}
	block, err := aes.NewCipher(cipherState.key)
	if err != nil {
		return nil, nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, err
	}
	ciphertext := gcm.Seal(nil, nonce, privateKey, []byte(agentID+":"+keyID))
	return ciphertext, nonce, nil
}

func (cipherState *keyCipher) decrypt(key SigningKey) (ed25519.PrivateKey, error) {
	if !cipherState.available() {
		return nil, ErrUnavailable
	}
	block, err := aes.NewCipher(cipherState.key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	plain, err := gcm.Open(nil, key.Nonce, key.EncryptedPrivateKey, []byte(key.AgentID+":"+key.ID))
	if err != nil {
		return nil, ErrUnavailable
	}
	if len(plain) != ed25519.PrivateKeySize {
		return nil, ErrUnavailable
	}
	return ed25519.PrivateKey(plain), nil
}

func keyFingerprint(publicKey ed25519.PublicKey) string {
	digest := sha256.Sum256(publicKey)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func signDocument(document Document, key SigningKey, privateKey ed25519.PrivateKey) (Document, string, string, string, error) {
	document.Proof = map[string]any{}
	payload, err := json.Marshal(document)
	if err != nil {
		return Document{}, "", "", "", err
	}
	digest := sha256.Sum256(payload)
	protectedJSON, _ := json.Marshal(map[string]string{"alg": "EdDSA", "kid": key.ID, "typ": "agentfacts+jws"})
	protected := base64.RawURLEncoding.EncodeToString(protectedJSON)
	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	signature := ed25519.Sign(privateKey, []byte(protected+"."+encodedPayload))
	encodedSignature := base64.RawURLEncoding.EncodeToString(signature)
	digestText := "sha256:" + hex.EncodeToString(digest[:])
	document.Proof = map[string]any{
		"type": "JsonWebSignature", "keyId": key.ID, "protected": protected,
		"signature": encodedSignature, "digest": digestText,
	}
	return document, digestText, protected, encodedSignature, nil
}
