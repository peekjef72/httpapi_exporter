package encrypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

type AESCipher struct {
	GCM       cipher.AEAD
	nonceSize int
}

// Initilze AES/GCM for both encrypting and decrypting.
func NewAESCipher(key_str string) (*AESCipher, error) {

	block, err := aes.NewCipher([]byte(key_str))
	if err != nil {
		return nil, fmt.Errorf("error reading key: %s", err.Error())
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("error initializing AEAD: %s", err.Error())
	}

	res := AESCipher{
		GCM:       gcm,
		nonceSize: gcm.NonceSize(),
	}
	return &res, nil
}

func randBytes(length int) []byte {
	b := make([]byte, length)
	rand.Read(b)
	return b
}

func (ci *AESCipher) Encrypt(plaintext []byte, base64_encoded bool) (cipherstring string) {
	nonce := randBytes(ci.nonceSize)
	ciphertext := ci.GCM.Seal(nonce, nonce, plaintext, nil)

	if base64_encoded {
		cipherstring = base64.StdEncoding.EncodeToString(ciphertext)
	} else {
		cipherstring = hex.EncodeToString(ciphertext)
	}
	return cipherstring
}

func (ci *AESCipher) Decrypt(cipherstring string, base64_encoded bool) (plainstring string, err error) {
	var ciphertext, plaintext []byte

	if base64_encoded {
		ciphertext, err = base64.StdEncoding.DecodeString(cipherstring)
	} else {
		ciphertext, err = hex.DecodeString(cipherstring)
	}
	if err != nil {
		return "", err
	}

	if len(ciphertext) < ci.nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce := ciphertext[0:ci.nonceSize]
	msg := ciphertext[ci.nonceSize:]
	plaintext, err = ci.GCM.Open(nil, nonce, msg, nil)
	if err != nil {
		return "", err
	}
	plainstring = string(plaintext)

	return plainstring, nil
}
