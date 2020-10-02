package signer

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/keyvault/v7.0/keyvault"
	"github.com/thalesgroup/sshizzle/internal/azure"
	"golang.org/x/crypto/ssh"
)

var supportedExtensions = []string{
	// "no-agent-forwarding",
	// "no-port-forwarding",
	// "no-pty",
	// "no-user-rc",
	// "no-x11-forwarding",
	"permit-agent-forwarding",
	"permit-port-forwarding",
	"permit-pty",
	"permit-user-rc",
	"permit-X11-forwarding",
}

// FunctionInvocation contains details about a specific invocation
// of an Azure Function
type FunctionInvocation struct {
	UserAgent           string
	InvocationID        string
	ClientPrincipalID   string
	ClientPrincipalName string
	ClientIP            string
}

// SignCertificate takes a public key and returns a signed SSH cert
func SignCertificate(invocationDetail *FunctionInvocation, keyvaultClient *keyvault.BaseClient, keyvaultName string, keyName string, pubKey ssh.PublicKey) (*ssh.Certificate, error) {
	// Generate a nonce
	bytes := make([]byte, 32)
	nonce := make([]byte, len(bytes)*2)
	if _, err := rand.Read(bytes); err != nil {
		return nil, err
	}
	hex.Encode(nonce, bytes)

	// Generate a random serial number
	serial, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
	if err != nil {
		return nil, err
	}

	// Set the certificate principal to the signed in user
	username := strings.Split(invocationDetail.ClientPrincipalName, "@")[0]

	// Get the current time and generate the validFrom and ValidTo
	now := time.Now()
	validFrom := now.Add(time.Second * -15)
	validTo := now.Add(time.Minute * 2)

	// Convert the extensions slice to a map
	extensions := make(map[string]string, len(supportedExtensions))
	for _, ext := range supportedExtensions {
		extensions[ext] = ""
	}

	criticalOptions := make(map[string]string)
	// criticalOptions["force-command"] = "echo Hello, SSHizzle!"
	// criticalOptions["source-address"] = "192.168.0.0/24"

	// Key ID to [loosely] follow Netflix BLESS format: https://github.com/Netflix/bless
	keyID := fmt.Sprintf("request[%s] for[%s] from[%s] command[%s] ssh_key[%s] ca[%s] valid_to[%s]",
		invocationDetail.InvocationID,
		username,
		invocationDetail.ClientIP,
		"", // Force command
		ssh.FingerprintSHA256(pubKey),
		os.Getenv("WEBSITE_DEPLOYMENT_ID"),
		validTo.Format("2006/01/02 15:04:05"),
	)
	// Create a certificate with all of our details
	certificate := ssh.Certificate{
		Nonce:    nonce,
		Key:      pubKey,
		Serial:   serial.Uint64(),
		CertType: ssh.UserCert,
		KeyId:    keyID,
		ValidPrincipals: []string{
			username,
		},
		Permissions: ssh.Permissions{
			CriticalOptions: criticalOptions,
			Extensions:      extensions,
		},
		ValidAfter:  uint64(validFrom.Unix()),
		ValidBefore: uint64(validTo.Unix()),
	}

	// Create a "KeyVaultSigner" which returns a crypto.Signer that interfaces with Azure Key Vault
	keyvaultSigner := azure.NewKeyVaultSigner(keyvaultClient, keyvaultName, keyName)

	// Create an SSHAlgorithmSigner with an RSA, SHA256 algorithm
	sshAlgorithmSigner, err := NewAlgorithmSignerFromSigner(keyvaultSigner, ssh.SigAlgoRSASHA2256)
	if err != nil {
		return nil, err
	}

	// Sign the certificate!
	if err := certificate.SignCert(rand.Reader, sshAlgorithmSigner); err != nil {
		return nil, err
	}

	// Extract the public key (certificate) to return to the user
	pubkey, err := ssh.ParsePublicKey(certificate.Marshal())
	if err != nil {
		return nil, err
	}

	// Convert the cert to the correct format and return it
	cert, _, _, _, err := ssh.ParseAuthorizedKey(ssh.MarshalAuthorizedKey(pubkey))
	if err != nil {
		return nil, err
	}

	return cert.(*ssh.Certificate), nil
}
