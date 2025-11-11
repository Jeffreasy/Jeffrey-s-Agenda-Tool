// Maak dit bestand: internal/crypto/crypto.go
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
	"os"
)

// Encrypt versleutelt data met AES-GCM
func Encrypt(plaintext []byte) ([]byte, error) {
	key := []byte(os.Getenv("ENCRYPTION_KEY"))
	if len(key) != 32 {
		return nil, fmt.Errorf("ENCRYPTION_KEY must be 32 bytes (AES-256)")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// De nonce (number used once) is cruciaal voor GCM.
	// We maken een nonce van de standaardgrootte.
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	// gcm.Seal versleutelt de data.
	// We slaan de nonce op VOORAAN de ciphertext.
	// Formaat: [nonce][versleutelde_data]
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt ontcijfert data met AES-GCM
func Decrypt(ciphertext []byte) ([]byte, error) {
	key := []byte(os.Getenv("ENCRYPTION_KEY"))
	if len(key) != 32 {
		return nil, fmt.Errorf("ENCRYPTION_KEY must be 32 bytes (AES-256)")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	// Haal de nonce (die we vooraan hadden opgeslagen) er weer af
	nonce, actualCiphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// gcm.Open ontcijfert en authenticeert de data
	plaintext, err := gcm.Open(nil, nonce, actualCiphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed: %v", err)
	}

	return plaintext, nil
}
