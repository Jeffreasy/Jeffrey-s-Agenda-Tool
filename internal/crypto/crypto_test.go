package crypto

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEncryptDecrypt(t *testing.T) {
	// Set up a test encryption key
	testKey := "12345678901234567890123456789012" // 32 bytes for AES-256
	t.Setenv("ENCRYPTION_KEY", testKey)

	originalText := []byte("Hello, World! This is a test message.")

	// Encrypt the data
	ciphertext, err := Encrypt(originalText)
	assert.NoError(t, err)
	assert.NotNil(t, ciphertext)
	assert.NotEqual(t, originalText, ciphertext)

	// Decrypt the data
	decryptedText, err := Decrypt(ciphertext)
	assert.NoError(t, err)
	assert.NotNil(t, decryptedText)
	assert.Equal(t, originalText, decryptedText)
}

func TestEncrypt_InvalidKeyLength(t *testing.T) {
	// Set up an invalid encryption key (too short)
	t.Setenv("ENCRYPTION_KEY", "short")

	originalText := []byte("test")

	// Encrypt should fail
	ciphertext, err := Encrypt(originalText)
	assert.Error(t, err)
	assert.Nil(t, ciphertext)
	assert.Contains(t, err.Error(), "ENCRYPTION_KEY must be 32 bytes")
}

func TestDecrypt_InvalidKeyLength(t *testing.T) {
	// Set up an invalid encryption key (too short)
	t.Setenv("ENCRYPTION_KEY", "short")

	ciphertext := []byte("fake ciphertext")

	// Decrypt should fail
	plaintext, err := Decrypt(ciphertext)
	assert.Error(t, err)
	assert.Nil(t, plaintext)
	assert.Contains(t, err.Error(), "ENCRYPTION_KEY must be 32 bytes")
}

func TestDecrypt_CiphertextTooShort(t *testing.T) {
	// Set up a valid encryption key
	testKey := "12345678901234567890123456789012"
	t.Setenv("ENCRYPTION_KEY", testKey)

	// Ciphertext that's too short (shorter than nonce size)
	ciphertext := []byte("short")

	plaintext, err := Decrypt(ciphertext)
	assert.Error(t, err)
	assert.Nil(t, plaintext)
	assert.Contains(t, err.Error(), "ciphertext too short")
}

func TestDecrypt_InvalidCiphertext(t *testing.T) {
	// Set up a valid encryption key
	testKey := "12345678901234567890123456789012"
	t.Setenv("ENCRYPTION_KEY", testKey)

	// Create some fake ciphertext that's long enough but invalid
	ciphertext := make([]byte, 100)
	for i := range ciphertext {
		ciphertext[i] = byte(i)
	}

	plaintext, err := Decrypt(ciphertext)
	assert.Error(t, err)
	assert.Nil(t, plaintext)
	assert.Contains(t, err.Error(), "decryption failed")
}

func TestEncryptDecrypt_EmptyData(t *testing.T) {
	// Set up a test encryption key
	testKey := "12345678901234567890123456789012"
	t.Setenv("ENCRYPTION_KEY", testKey)

	originalText := []byte("")

	// Encrypt the empty data
	ciphertext, err := Encrypt(originalText)
	assert.NoError(t, err)
	assert.NotNil(t, ciphertext)

	// Decrypt the data
	decryptedText, err := Decrypt(ciphertext)
	assert.NoError(t, err)
	// AES-GCM returns nil for empty plaintext, so we check it's empty
	assert.Len(t, decryptedText, 0)
}

func TestEncryptDecrypt_LargeData(t *testing.T) {
	// Set up a test encryption key
	testKey := "12345678901234567890123456789012"
	t.Setenv("ENCRYPTION_KEY", testKey)

	// Create a large test message
	originalText := make([]byte, 10000)
	for i := range originalText {
		originalText[i] = byte(i % 256)
	}

	// Encrypt the data
	ciphertext, err := Encrypt(originalText)
	assert.NoError(t, err)
	assert.NotNil(t, ciphertext)

	// Decrypt the data
	decryptedText, err := Decrypt(ciphertext)
	assert.NoError(t, err)
	assert.Equal(t, originalText, decryptedText)
}
