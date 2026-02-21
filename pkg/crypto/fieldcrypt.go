// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package crypto provides AES-256-GCM field-level encryption for LGPD Art. 46 compliance.
// Sensitive fields (CPF, nome, telefone, clinical data) are encrypted at rest in PostgreSQL.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"regexp"
	"sync"
)

const encPrefix = "enc::"

var (
	gcmInstance cipher.AEAD
	initOnce    sync.Once
	initErr     error
)

// Init initializes the encryption module. Must be called once at startup.
// Reads ENCRYPTION_KEY from environment (base64-encoded, 32 bytes decoded).
func Init() error {
	initOnce.Do(func() {
		keyStr := os.Getenv("ENCRYPTION_KEY")
		if keyStr == "" {
			initErr = fmt.Errorf("ENCRYPTION_KEY nao configurada — dados sensiveis nao serao criptografados")
			return
		}

		key, err := base64.StdEncoding.DecodeString(keyStr)
		if err != nil {
			initErr = fmt.Errorf("ENCRYPTION_KEY base64 invalida: %w", err)
			return
		}
		if len(key) != 32 {
			initErr = fmt.Errorf("ENCRYPTION_KEY deve ter 32 bytes (AES-256), tem %d", len(key))
			return
		}

		block, err := aes.NewCipher(key)
		if err != nil {
			initErr = fmt.Errorf("falha ao criar AES cipher: %w", err)
			return
		}

		gcmInstance, err = cipher.NewGCM(block)
		if err != nil {
			initErr = fmt.Errorf("falha ao criar GCM: %w", err)
			return
		}
	})
	return initErr
}

// IsEnabled returns true if encryption was successfully initialized.
func IsEnabled() bool {
	return gcmInstance != nil
}

// Encrypt encrypts plaintext using AES-256-GCM and returns a prefixed base64 string.
// If encryption is not initialized, returns the plaintext unchanged.
func Encrypt(plaintext string) string {
	if plaintext == "" || gcmInstance == nil {
		return plaintext
	}

	nonce := make([]byte, gcmInstance.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return plaintext
	}

	ciphertext := gcmInstance.Seal(nonce, nonce, []byte(plaintext), nil)
	return encPrefix + base64.StdEncoding.EncodeToString(ciphertext)
}

// Decrypt decrypts an encrypted string. If the string is not prefixed with "enc::",
// it is returned as-is (backward compatibility with unencrypted data).
func Decrypt(ciphertext string) string {
	if ciphertext == "" || gcmInstance == nil {
		return ciphertext
	}

	if len(ciphertext) <= len(encPrefix) || ciphertext[:len(encPrefix)] != encPrefix {
		return ciphertext // not encrypted, return as-is
	}

	data, err := base64.StdEncoding.DecodeString(ciphertext[len(encPrefix):])
	if err != nil {
		return ciphertext
	}

	nonceSize := gcmInstance.NonceSize()
	if len(data) < nonceSize {
		return ciphertext
	}

	nonce, encrypted := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcmInstance.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return ciphertext
	}

	return string(plaintext)
}

var nonDigitRe = regexp.MustCompile(`\D`)

// HashCPF returns a deterministic SHA-256 hex hash of a CPF (digits only).
// Used for indexed lookups on encrypted CPF columns.
func HashCPF(cpf string) string {
	digits := nonDigitRe.ReplaceAllString(cpf, "")
	h := sha256.Sum256([]byte(digits))
	return hex.EncodeToString(h[:])
}
