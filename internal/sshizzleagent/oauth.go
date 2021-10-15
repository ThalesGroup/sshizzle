package sshizzleagent

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"time"

	"github.com/google/uuid"
	"github.com/thalesgroup/sshizzle/internal/config"
	"golang.org/x/oauth2"
)

// Authenticate takes an OAuth2 token and validates it. If invalid, it attempts to authenticate and renew
func Authenticate(token *oauth2.Token, config *oauth2.Config) (*oauth2.Token, error) {
	// Check if the token we already have is valid, if not, fetch a new one
	if !token.Valid() {
		var server *http.Server = &http.Server{}
		// Create a state for later validation
		state := uuid.New().String()

		// Start the calback handler
		go func() {
			mux := http.NewServeMux()
			mux.HandleFunc("/callback", handleLoginCallback(token, config, state))
			server = &http.Server{Addr: ":8080", Handler: mux}
			if err := server.ListenAndServe(); err != nil {
				log.Println(fmt.Errorf("error in callback listener: %s", err.Error()))
			}
		}()
		// Close the callback handler on function return
		defer server.Close()

		// Get the URL required for the user to authenticate
		url := config.AuthCodeURL(state, oauth2.AccessTypeOffline)
		// Try to open the URL in the browser
		err := openURL(url)
		if err != nil {
			// Otherwise dump the URL to stdout as a prompt
			log.Printf("Failed to open browser. Please visit this URL and sign in:\n\n%s\n\n", url)
			log.Println("Waiting up to 60s for authentication...")
		}

		// Catch interrupt/kill signals to exit nicely
		sigs := make(chan os.Signal)
		signal.Notify(sigs, os.Interrupt, os.Kill)

		// Take the current time
		now := time.Now()
		// Wait for the login callback to succeed or timeout
		for !token.Valid() {
			// Check if we've been waiting longer than 60s
			if time.Now().Unix() > now.Add(60*time.Second).Unix() {
				return nil, fmt.Errorf("azure AD authentication timed out after 60s, try again")
			}
			// Check for user input/interrupt
			select {
			case s := <-sigs:
				return nil, fmt.Errorf("Cancelled, signal received: %s", s.String())
			default:
				time.Sleep(time.Second * 1)
			}
		}
	}
	return token, nil
}

// Handler for the response from requesting a new token
func handleLoginCallback(token *oauth2.Token, oauth *oauth2.Config, state string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// We'll output some basic HTML to the user, so set the header accordingly
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		// Check the authcode matches the last login request
		if r.FormValue("state") != state {
			fmt.Fprintf(w, "<h3>Oops, that didn't work! AuthCode invalid!</h3>")
			return
		}
		// Retreive the auth code from the response
		code := r.FormValue("code")
		// Attempt to exchange the auth code for a new token
		newToken, err := oauth.Exchange(context.Background(), code)

		if err != nil {
			fmt.Fprintf(w, "<h3>Oops, that didn't work! Try again?</h3>")
			return
		}
		fmt.Fprintf(w, "<h3>Success! You can close this window now!</h3>")

		// Update the original token value
		*token = *newToken

		// Convert the response to [nice, indented] JSON
		json, err := json.MarshalIndent(token, "", "  ")
		if err != nil {
			return
		}

		// Get path to the token cache
		tokenFile, err := config.GetSSHizzleTokenFile()
		if err == nil {
			// If we got the path successfully, try to write the file
			if err = ioutil.WriteFile(tokenFile, json, 0600); err != nil {
				log.Printf("unable to update token cache at %s\n", tokenFile)
			}
		}
	}
}

// openURL attempts to open a URL in a browser in an OS-agnostic way
// #nosec
func openURL(url string) (err error) {
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	if err != nil {
		return err
	}
	return nil
}
