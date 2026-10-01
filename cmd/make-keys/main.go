package main

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
)

func main() {
	// Generate a new Ed25519 key pair
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		panic(err)
	}
	
	// Convert keys to hex strings
	publicKeyHex := hex.EncodeToString(publicKey)
	privateKeyHex := hex.EncodeToString(privateKey)
	fmt.Println("Public Key:", publicKeyHex)
	fmt.Println("Private Key:", privateKeyHex)
}
