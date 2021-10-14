package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Azure/go-autorest/autorest/adal"
)

// MSIResourceToken represents an Identity Token provided by
// an Azure Managed Service Identity
type MSIResourceToken struct {
	AccessToken string      `json:"access_token"`
	ExpiresOn   json.Number `json:"expires_on"`
	Resource    string      `json:"resource"`
	TokenType   string      `json:"token_type"`
	ClientID    string      `json:"client_id"`
}

// GetServicePrincipalTokenFromMSI gets a standard Service Principal Token from a Managed Service Identity that's
// assigned to an Azure Function.
func GetServicePrincipalTokenFromMSI(ctx context.Context, endpoint string) (*adal.ServicePrincipalToken, error) {
	// Retreive the MSI endpoint for the Azure Function
	// Azure Go SDK method for creating Authorizers from MSI doesn't work in functions
	// https://docs.microsoft.com/en-us/azure/app-service/overview-managed-identity?tabs=javascript
	idEndpoint, err := adal.GetMSIAppServiceEndpoint()
	if err != nil {
		return &adal.ServicePrincipalToken{}, fmt.Errorf("failed to retreive MSI endpoint: %s", err.Error())
	}

	// Fetch Identity Header for increased CSRF protection
	// https://docs.microsoft.com/en-us/azure/app-service/overview-managed-identity?tabs=dotnet#using-the-rest-protocol
	idHeader, exists := os.LookupEnv("IDENTITY_HEADER")
	if !exists {
		return &adal.ServicePrincipalToken{}, fmt.Errorf("failed to retreive MSI Identity Header: %s", err.Error())
	}

	// Specify the MSI token endpoint API version
	apiVersion := "2019-08-01"

	// Create the URL to request an access token for the keyvault endpoint
	url := fmt.Sprintf("%s?resource=%s&api-version=%s", idEndpoint, endpoint, apiVersion)

	// Create an HTTP client and set appropriate headers for token request
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("X-IDENTITY-HEADER", idHeader)

	// Make the request for a token
	res, err := client.Do(req)
	if err != nil {
		return &adal.ServicePrincipalToken{}, fmt.Errorf("failed token request with URL %s: %s", url, err.Error())
	}

	// Read the response and fail out if required
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return &adal.ServicePrincipalToken{}, fmt.Errorf("failed to read token request response body: %s", err.Error())
	}

	// Define an MSIResourceToken to unmarshal response into
	var msiToken MSIResourceToken
	err = json.Unmarshal(body, &msiToken)
	if err != nil {
		return &adal.ServicePrincipalToken{}, fmt.Errorf("failed to unmarshal response into MSIResponseToken: %s", err.Error())
	}

	// Fetch the Azure AD Tenant ID from environment variable
	tenantID := strings.Split(os.Getenv("WEBSITE_AUTH_OPENID_ISSUER"), "/")[3]

	// Create a new OAuthConfig
	oauthConfig, err := adal.NewOAuthConfig(endpoint, tenantID)
	if err != nil {
		return &adal.ServicePrincipalToken{}, fmt.Errorf("failed to create new OAuth config: %s", err.Error())
	}

	// Fetch a service principal token using the MSI token retrieved from MSI endpoint
	spToken, err := adal.NewServicePrincipalTokenFromManualToken(*oauthConfig, msiToken.ClientID, msiToken.Resource, adal.Token{
		AccessToken: msiToken.AccessToken,
		Resource:    msiToken.Resource,
		ExpiresOn:   msiToken.ExpiresOn,
		Type:        msiToken.TokenType,
	}, nil)

	if err != nil {
		return &adal.ServicePrincipalToken{}, fmt.Errorf("failed to create Service Principal token: %s", err.Error())
	}

	return spToken, nil
}
