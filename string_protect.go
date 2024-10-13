package main

import (
	"crypto/sha256"
	"fmt"
)

const HASH_MAGIC_SALT = "lXz0r0oj"

var protectedNum = 0

func createProtectedStringFunc(str string) (name string, decl string) {
	// Uses hash + an incremented num to ensure unique name
	hash := shortHash(str + HASH_MAGIC_SALT)
	name = "l" + hash + fmt.Sprint(protectedNum)
	protectedNum++

	decl = "func " + name + "() string {\n"

	bytes := []byte(str)
	decl += "	b := []byte{"
	for i, b := range bytes {
		decl += fmt.Sprint(b+(byte(i)*5)) + ","
	}
	// Remove final comma
	decl = decl[:len(decl)-1]
	decl += "}\n"

	decl += "	return string([]byte{"
	for i := range len(bytes) {
		decl += "b[" + fmt.Sprint(i) + "] - " + fmt.Sprint(i*5) + ","
	}
	// Remove final comma
	decl = decl[:len(decl)-1]
	decl += "})\n"

	decl += "}"
	return
}

func shortHash(value string) string {
	// Create a SHA-256 hash of the input value
	hash := sha256.Sum256([]byte(value))

	// Convert the hash to a hexadecimal string
	hexHash := fmt.Sprintf("%x", hash)

	// Take the first 8 characters of the hexadecimal hash
	shortHash := hexHash[:8]

	return shortHash
}
