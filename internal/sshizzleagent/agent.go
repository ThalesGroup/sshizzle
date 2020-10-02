package sshizzleagent

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"path/filepath"
	"time"

	"github.com/thalesgroup/sshizzle/internal/azure"
	"github.com/thalesgroup/sshizzle/internal/config"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/oauth2"
)

type sshizzleAgent struct {
	signer      ssh.Signer
	certificate *ssh.Certificate
	config      *config.SSHizzleConfig
	token       *oauth2.Token
	state       string
}

// NewSSHizzleAgent returns a new Agent with a signer and cert
func NewSSHizzleAgent(c *config.SSHizzleConfig) agent.Agent {
	// Create a pointer to an empty (invalid) token
	token := &oauth2.Token{}
	// Get the path to the sshizzle token cache
	tokenFile, err := config.GetSSHizzleTokenFile()
	if err == nil {
		// Read the file if we got the path okay
		data, err := ioutil.ReadFile(filepath.Clean(tokenFile))
		if err == nil {
			// If we were able to read the file, unmarshal it into `token`
			err = json.Unmarshal(data, token)
		}
	}

	// Return a new sshizzleAgent
	return &sshizzleAgent{
		signer:      c.Signer,
		certificate: &ssh.Certificate{},
		config:      c,
		state:       "",
		token:       token,
	}
}

// RemoveAll clears the current certificate and identity token (including refresh token)
func (a *sshizzleAgent) RemoveAll() error {
	a.certificate = &ssh.Certificate{}
	a.token = &oauth2.Token{}
	return nil
}

// Remove has the same functionality as RemoveAll
func (a *sshizzleAgent) Remove(key ssh.PublicKey) error {
	return a.RemoveAll()
}

// List returns the identities, but also signs the certificate using sshizzle-ca if expired.
func (a *sshizzleAgent) List() ([]*agent.Key, error) {
	now := time.Now().Unix()
	before := int64(a.certificate.ValidBefore)
	var ids []*agent.Key
	// Print something in the log so we can check the agent is being used
	log.Println("Agent invoked, trying to get credentials")

	// Check if certificate is valid, if not, try to renew it
	if a.certificate.ValidBefore != uint64(ssh.CertTimeInfinity) && (now >= before || before < 0) {
		// Validate our current token, and request a new one if its invalid
		token, err := Authenticate(a.token, a.config.OauthConfig)
		if err != nil {
			return ids, err
		}
		// Refresh the agent's token in case it changed
		a.token = token

		// Get the public key of this agent's signer
		publicKey := a.signer.PublicKey()
		// Invoke the Azure Function to get (hopefully) a signed certificate!
		certificate, err := azure.InvokeSignFunction(&publicKey, a.config.FuncHost, a.config.OauthConfig, a.token)
		if err != nil {
			return ids, err
		}
		// Output the key ID to the logs
		log.Printf("New certificate acquired with ID: %s", certificate.KeyId)
		// Update the agent's stored certificate
		a.certificate = certificate
	}
	// Setup the list of identities and return it
	ids = append(ids, &agent.Key{
		Format:  a.certificate.Type(),
		Blob:    a.certificate.Marshal(),
		Comment: a.certificate.KeyId,
	})
	return ids, nil
}

// Signs a challenge required to authenticate with an SSH host
func (a *sshizzleAgent) Sign(key ssh.PublicKey, data []byte) (*ssh.Signature, error) {
	return a.signer.Sign(rand.Reader, data)
}

// Signers list our current signers which there is only one.
func (a *sshizzleAgent) Signers() ([]ssh.Signer, error) {
	return []ssh.Signer{
		a.signer,
	}, nil
}

// ErrUnsupported is a generic error to be returned when unsupported agent methods are called
var ErrUnsupported = errors.New("action not supported by sshizzle-agent")

// Unsupported
func (a *sshizzleAgent) Lock(passphrase []byte) error {
	return ErrUnsupported
}

// Unsupported
func (a *sshizzleAgent) Unlock(passphrase []byte) error {
	return ErrUnsupported
}

// Unsupported
func (a *sshizzleAgent) Add(key agent.AddedKey) error {
	return ErrUnsupported
}
