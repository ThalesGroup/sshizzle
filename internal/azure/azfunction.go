package azure

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"golang.org/x/crypto/ssh"
	"golang.org/x/oauth2"
)

// FunctionPayload is the payload structure for the Azure Function
type FunctionPayload struct {
	PublicKey string `json:"public_key"`
}

// FunctionResponse is the structure for a response from the Azure Function
type FunctionResponse struct {
	Response string `json:"response"`
}

// InvokeSignFunction invokes the sshizzle-ca on Azure Functions with a given OAuth config and token
func InvokeSignFunction(publicKey *ssh.PublicKey, funcHost string, oauthConfig *oauth2.Config, token *oauth2.Token) (*ssh.Certificate, error) {
	// Construct function URL from Function Name
	funcURL := "https://" + funcHost + "/api/sign-agent-key"

	// Marshal Public Key and encode into Base64
	encodedKey := base64.RawURLEncoding.EncodeToString((*publicKey).Marshal())

	// Create a function payload containing the key
	payload := &FunctionPayload{encodedKey}

	// Create a marhsalled payload
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	// Setup the POST request
	request, err := http.NewRequest("POST", funcURL, bytes.NewBuffer(jsonPayload))
	request.Header.Set("Content-Type", "application/json")

	// Create a client using the OAuth token we fetched earlier
	client := oauthConfig.Client(context.Background(), token)

	// Invoke the function
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}

	// Read the whole response
	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)

	// Check if request was successful
	if response.StatusCode >= 200 && response.StatusCode <= 299 {
		// Unmarshal and decode
		result := &FunctionResponse{}
		err = json.Unmarshal(body, &result)
		if err != nil {
			return nil, err
		}

		// Decode the certificate from Base64
		decoded, err := base64.RawURLEncoding.DecodeString(result.Response)
		if err != nil {
			return nil, err
		}

		// Unmarshal the decoded certificate into an ssh.Certificate
		pubkey, err := ssh.ParsePublicKey(decoded)
		if err != nil {
			return nil, err
		}

		// Return the certificate to the caller!
		return pubkey.(*ssh.Certificate), nil
	}

	if response.StatusCode == 404 {
		return nil, fmt.Errorf("azure function invocation failed with error 404, try clearing your DNS cache and try again")
	}

	return nil, fmt.Errorf("azure function invocation failed with %d, %s", response.StatusCode, string(body))
}
