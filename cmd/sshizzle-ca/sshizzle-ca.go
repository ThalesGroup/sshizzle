package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/keyvault/v7.0/keyvault"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure"
	az "github.com/thalesgroup/sshizzle/internal/azure"
	"github.com/thalesgroup/sshizzle/internal/signer"
	"golang.org/x/crypto/ssh"
)

func httpTriggerHandler(ctx context.Context) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get details of this function invocation
		invocationDetail := signer.FunctionInvocation{
			UserAgent:           r.Header.Get("User-Agent"),
			InvocationID:        r.Header.Get("X-Azure-Functions-InvocationId"),
			ClientPrincipalID:   r.Header.Get("X-Ms-Client-Principal-Id"),
			ClientPrincipalName: r.Header.Get("X-Ms-Client-Principal-Name"),
			ClientIP:            strings.Split(r.Header.Get("X-Forwarded-For"), ":")[0],
		}

		// Initialise a payload object to parse the JSON payload
		payload := &az.FunctionPayload{}

		// Decode the JSON body into our payload
		err := json.NewDecoder(r.Body).Decode(&payload)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Decode the public key from base64 (URL encoding)
		decoded, err := base64.RawURLEncoding.DecodeString(payload.PublicKey)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Create a PublicKey from the payload
		publicKey, err := ssh.ParsePublicKey(decoded)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Trim the keyvault endpoint to remove the trailing slash
		keyvaultEndpoint := strings.TrimSuffix(azure.PublicCloud.KeyVaultEndpoint, "/")

		// Get a service principal token from the MSI valid against the keyvault endpoint
		spToken, err := az.GetServicePrincipalTokenFromMSI(ctx, keyvaultEndpoint)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Create an authorizer using the new Service Principal token
		authorizer := autorest.NewBearerAuthorizer(spToken)

		// Create a KeyVault client and assign an authorizer
		kvClient := keyvault.New()
		kvClient.Authorizer = authorizer

		// Go and sign our public key!
		keyvaultName := os.Getenv("KV_NAME")
		signed, err := signer.SignCertificate(&invocationDetail, &kvClient, keyvaultName, "sshizzle", publicKey)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Create the response to the request
		funcResponse := az.FunctionResponse{
			Response: base64.RawURLEncoding.EncodeToString(signed.Marshal()),
		}

		// Marshal response into JSON
		js, err := json.Marshal(funcResponse)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Set the content-type and write the response
		w.Header().Set("Content-Type", "application/json")
		_, err = w.Write(js)
		if err != nil {
			log.Println(fmt.Errorf("error writing response: %s", err.Error()))
		}
	}
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	httpInvokerPort, exists := os.LookupEnv("FUNCTIONS_HTTPWORKER_PORT")
	if exists {
		log.Printf("FUNCTIONS_HTTPWORKER_PORT: %s\n", httpInvokerPort)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/sign-agent-key", httpTriggerHandler(ctx))

	server := &http.Server{
		Addr:           ":" + httpInvokerPort,
		Handler:        mux,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	idleConnsClosed := make(chan struct{})
	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt, syscall.SIGTERM)

		select {
		case <-ctx.Done():
		case <-ch:
			cancel()
		}

		// We received an interrupt signal, shut down
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Fatalf("HTTP server shutdown error: %v\n", err)
		}
		shutdownCancel()
		close(idleConnsClosed)
	}()

	log.Println("Go server Listening...on httpInvokerPort:", httpInvokerPort)
	err := server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		log.Fatalln(err)
	}
	<-idleConnsClosed
}
