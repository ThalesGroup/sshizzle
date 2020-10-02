package azure

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"math/big"

	"github.com/Azure/azure-sdk-for-go/services/keyvault/v7.0/keyvault"
	"github.com/pkg/errors"
)

// KeyVaultSigner an Azure Key Vault signer
type KeyVaultSigner struct {
	crypto.Signer
	client *keyvault.BaseClient
	url    string
	key    string
}

// NewKeyVaultSigner returns a new instance of a KeyVaultSigner
func NewKeyVaultSigner(client *keyvault.BaseClient, keyVaultName string, key string) *KeyVaultSigner {
	// Construct the URL to the keyvault from the name
	keyvaultURL := fmt.Sprintf("https://%s.vault.azure.net/", keyVaultName)
	// Return a new KeyVaultSigner
	return &KeyVaultSigner{
		client: client,
		url:    keyvaultURL,
		key:    key,
	}
}

// Public returns the PublicKey from an Azure Key Vault Key
func (s *KeyVaultSigner) Public() crypto.PublicKey {
	// Get the key from Azure Key Vault
	keyBundle, err := s.client.GetKey(context.Background(), s.url, s.key, "")
	if err != nil {
		return nil
	}

	// Retreive the key modulus and decode from Base64
	keyModulus, err := base64.RawURLEncoding.DecodeString(*keyBundle.Key.N)
	if err != nil {
		return nil
	}

	// Retrieve the key exponent and decode from Bae64
	keyExponent, err := base64.RawURLEncoding.DecodeString(*keyBundle.Key.E)
	if err != nil {
		return nil
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
		return nil
	}

	// Create a new PublicKey using our computed values
	key := rsa.PublicKey{N: n, E: int(e)}

	// Convert into the rigth format
	publicKey, err := x509.ParsePKCS1PublicKey(x509.MarshalPKCS1PublicKey(&key))
	if err != nil {
		return nil
	}

	return publicKey
}

// Sign a digest with the private key in Azure Key Vault
func (s *KeyVaultSigner) Sign(rand io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	// Encode the digest into URL-encoded base64
	encodedDigest := base64.RawURLEncoding.EncodeToString(digest)

	// Attempt to sign using the KeyVault client
	response, err := s.client.Sign(
		context.Background(),
		s.url,
		s.key,
		"",
		keyvault.KeySignParameters{
			Algorithm: keyvault.RS256,
			Value:     &encodedDigest,
		},
	)

	// Parse the result, decoding from base64
	signature, err := base64.RawURLEncoding.DecodeString(*response.Result)
	if err != nil {
		return nil, errors.New("failed to decode signature result from Azure Function")
	}

	// Success!
	return signature, nil
}
