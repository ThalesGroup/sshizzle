package main

import (
	"bytes"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"os"

	"golang.org/x/crypto/ssh"
)

func main() {
	in := struct {
		N string `json:"n"`
		E string `json:"e"`
	}{}
	err := json.NewDecoder(os.Stdin).Decode(&in)
	if err != nil {
		log.Fatal(fmt.Errorf("invalid input: %w", err))
	}

	// Retreive the key modulus and decode from Base64
	keyModulus, err := base64.RawURLEncoding.DecodeString(in.N)
	if err != nil {
		log.Fatal(fmt.Errorf("invalid key modulus(N): %w", err))
	}

	// Retrieve the key exponent and decode from Bae64
	keyExponent, err := base64.RawURLEncoding.DecodeString(in.E)
	if err != nil {
		log.Fatal(fmt.Errorf("invalid key exponent(E): %w", err))
	}

	// Create the modulus big number
	n := big.NewInt(0)
	n.SetBytes(keyModulus)

	// Create the exponent byte array
	var eBytes []byte
	if len(keyExponent) < 8 {
		eBytes = make([]byte, 8-len(keyExponent), 8)
		eBytes = append(eBytes, keyExponent...)
	} else {
		eBytes = keyExponent
	}

	// Read the exponent in big endian binary format into a variabele
	eReader := bytes.NewReader(eBytes)
	var e uint64
	err = binary.Read(eReader, binary.BigEndian, &e)
	if err != nil {
		log.Fatal(err)
	}

	// Create a new PublicKey using our computed values
	key := rsa.PublicKey{N: n, E: int(e)}

	// Convert into the rigth format
	publicKey, err := x509.ParsePKCS1PublicKey(x509.MarshalPKCS1PublicKey(&key))
	if err != nil {
		log.Fatal(err)
	}

	// Create ssh public key from PKCS1 public key
	sshKey, err := ssh.NewPublicKey(publicKey)
	if err != nil {
		log.Fatal(err)
	}

	// Dump key to stdout as json
	sshKeyOutput := string(ssh.MarshalAuthorizedKey(sshKey))
	json.NewEncoder(os.Stdout).Encode(struct {
		CA string `json:"ca"`
	}{
		CA: sshKeyOutput,
	})
}
