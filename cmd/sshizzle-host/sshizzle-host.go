package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/keyvault/v7.0/keyvault"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	az "github.com/thalesgroup/sshizzle/internal/azure"
	"golang.org/x/crypto/ssh"
)

var kvResource string = strings.Trim(azure.PublicCloud.KeyVaultEndpoint, "/")

func main() {
	var keyvaultName string
	flag.StringVar(&keyvaultName, "kvName", "kv-sshizzle", "specify the keyvault name")
	flag.Parse()

	if os.Getenv("KV_NAME") != "" {
		keyvaultName = os.Getenv("KV_NAME")
	}

	// Setup an authoriser for KeyVault resources using the users credentials from
	// the Azure CLI
	var authorizer autorest.Authorizer
	var err error

	// Check if we've got an MSI, and use it if we do
	if checkMSI() {
		msiConf := auth.NewMSIConfig()
		msiConf.Resource = kvResource
		authorizer, err = msiConf.Authorizer()
	} else {
		authorizer, err = auth.NewAuthorizerFromCLIWithResource(kvResource)
	}
	// We need to exit if this doesn't work!
	if err != nil {
		log.Fatalln("Unable to authorize access to KeyVault service using `az` CLI credentials, please ensure you're logged in with `az account list` or there is an MSI present")
	}
	// Setup a KeyVault client
	kvClient := keyvault.New()
	kvClient.Authorizer = authorizer
	// Create a KeyVault Signer
	kvSigner := az.NewKeyVaultSigner(&kvClient, keyvaultName, "sshizzle")
	// Get the crypto.PublicKey back from the Signer
	publicKey := kvSigner.Public()
	// Convert to an SSH public key
	sshKey, err := ssh.NewPublicKey(publicKey)
	if err != nil {
		log.Fatalln("No public key received from key vault")
	}
	// Dump key to stdout in correct format
	sshKeyOutput := string(ssh.MarshalAuthorizedKey(sshKey))
	// Write some output to give the user a warm-fuzzy feeling
	log.Printf("Got CA public key:\n\n%s\n", sshKeyOutput)
	log.Printf("Writing key to `/etc/ssh/user_ca.pub`")
	// Write the CA public key to a sensible location
	keyFile := "/etc/ssh/user_ca.pub"
	// #nosec
	if err = ioutil.WriteFile(keyFile, ssh.MarshalAuthorizedKey(sshKey), 0644); err != nil {
		log.Printf("unable to write sshizzle-ca public key to %s\n", keyFile)
	}
	log.Printf("Key written, checking sshd config...")
	// We need to add a line to the sshd config for it to use the CA
	sshdConfig, err := ioutil.ReadFile("/etc/ssh/sshd_config")
	if err != nil {
		log.Fatalln("Unable to read `/etc/ssh/sshd_config`")
	}
	caConfigLine := "TrustedUserCAKeys /etc/ssh/user_ca.pub"
	// Check if the SSH daemon has already been configured
	configured := strings.Contains(string(sshdConfig), caConfigLine)
	if configured {
		log.Println("SSH daemon already configured")
	} else {
		// #nosec
		f, err := os.OpenFile("/etc/ssh/sshd_config", os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatalln("Unable to open `/etc/ssh/sshd_config` for appending")
		}
		// #nosec
		defer f.Close()
		if _, err := f.WriteString(fmt.Sprintf("%s\n", caConfigLine)); err != nil {
			log.Fatalln("Unable to append to `/etc/ssh/sshd_config`")
		}
		log.Println("SSH daemon configured.")
		log.Println("Restarting SSH Daemon")

		err = exec.Command("systemctl", "restart", "sshd").Run()
		if err != nil {
			log.Println("Couldn't restart SSH Daemon, try it yourself...")
		}
		log.Println("SSH Daemon restarted")
	}
	log.Println("Done")
	os.Exit(0)
}

// Function to check whether or not this is being run on a machine with an Azure Managed System Identity
func checkMSI() bool {
	// This URL should return a token if there is an MSI
	msiURL := "http://169.254.169.254/metadata/identity/oauth2/token"
	// Setup the request
	req, err := http.NewRequest("GET", msiURL, nil)
	if err != nil {
		return false
	}
	// Add some query parameters to the URL
	q := req.URL.Query()
	q.Add("resource", kvResource)
	q.Add("api-version", "2018-02-01")
	req.URL.RawQuery = q.Encode()
	// Add the metadata header to the request
	req.Header.Add("Metadata", "true")
	// Setup a client and do the request
	client := &http.Client{Timeout: time.Second * 2}
	res, err := client.Do(req)
	if err != nil {
		return false
	}
	// Check status code - if 200 we have an MSI
	if res.StatusCode > 299 {
		return false
	}
	return true
}
