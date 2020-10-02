package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"golang.org/x/crypto/ssh"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/microsoft"
)

// SSHizzleConfig contains information required to authenticate
// with Azure AD and invoke the lambda function
type SSHizzleConfig struct {
	Socket      string
	TenantID    string
	ClientID    string
	FuncHost    string
	Signer      ssh.Signer
	OauthConfig *oauth2.Config
}

// Check gets config from environment variables and creates sshizzle config dir
func Check() (*SSHizzleConfig, error) {
	err := godotenv.Load()
	if err != nil {
		fmt.Println(err.Error())
		return nil, err
	}
	// Read environment variables and fail out if not set
	funcHost, err := GetEnv("AZ_FUNC_HOST")
	if err != nil {
		return nil, err
	}
	tenantID, err := GetEnv("AZ_TENANT_ID")
	if err != nil {
		return nil, err
	}
	clientID, err := GetEnv("AZ_CLIENT_ID")
	if err != nil {
		return nil, err
	}

	// Get the default user config directory ($HOME/.config) on Linux
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	sshizzleDir := fmt.Sprintf("%s/%s", configDir, "sshizzle")

	// Check if the directory already exists, if not, create it
	if _, err := os.Stat(sshizzleDir); os.IsNotExist(err) {
		if err := os.MkdirAll(sshizzleDir, 0700); err != nil {
			return nil, err
		}
	}

	// Create a new SSHizzleConfig with the details specified
	config := SSHizzleConfig{
		Socket:   "/tmp/sshizzle.sock",
		TenantID: tenantID,
		ClientID: clientID,
		FuncHost: funcHost,
		Signer:   nil,
		OauthConfig: &oauth2.Config{
			RedirectURL:  "http://localhost:8080/callback",
			ClientID:     clientID,
			ClientSecret: "",
			Scopes:       []string{"openid offline_access https://" + funcHost + "/user_impersonation"},
			Endpoint:     microsoft.AzureADEndpoint(tenantID),
		},
	}

	return &config, nil
}

// GetSSHizzleDir returns the path to the sshizzle config dir
func GetSSHizzleDir() (string, error) {
	// Get the default user config directory ($HOME/.config) on Linux
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("error finding user config dir path: %s", err.Error())
	}
	sshizzleDir := fmt.Sprintf("%s/%s", configDir, "sshizzle")
	return sshizzleDir, nil
}

// GetSSHizzleTokenFile returns the filename of the token cache
func GetSSHizzleTokenFile() (string, error) {
	sshizzleDir, err := GetSSHizzleDir()
	if err != nil {
		return "", fmt.Errorf("error getting sshizzle token file path: %s", err.Error())
	}
	tokenFile := fmt.Sprintf("%s/%s", sshizzleDir, "token.json")
	return tokenFile, nil
}

// GetEnv gets an environment variable or returns an error if unset
func GetEnv(key string) (string, error) {
	result, exists := os.LookupEnv(key)
	if !exists {
		return "", fmt.Errorf("enviroment variable '%s' not set", key)
	}
	return result, nil
}
