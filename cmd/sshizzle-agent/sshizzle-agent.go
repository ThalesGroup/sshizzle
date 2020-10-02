package main

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/thalesgroup/sshizzle/internal/config"
	"github.com/thalesgroup/sshizzle/internal/sshizzleagent"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

func main() {
	// Ensure environment variables are set and config directory created
	config, err := config.Check()
	if err != nil {
		log.Fatalln(err)
	}

	// Generate a new SSH public/private key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Fatalln(fmt.Errorf("error generating new private key: %s", err.Error()))
	}

	// Create a signer from the key
	signer, err := ssh.NewSignerFromKey(privateKey)
	if err != nil {
		log.Fatalln(fmt.Errorf("error creating signer from private key: %s", err.Error()))
	}
	config.Signer = signer

	if err = startAgent(config); err != nil {
		log.Fatalln(fmt.Errorf("failed to start agent: %s", err.Error()))
	}
}

func startAgent(c *config.SSHizzleConfig) error {
	// Ensure the socket doesn't already exist
	if _, err := os.Stat(c.Socket); err == nil {
		log.Fatalln(fmt.Errorf("socket %s already exists", c.Socket))
	}

	// Create a new Unix socket and listen
	syscall.Umask(0077)
	listener, err := net.Listen("unix", c.Socket)
	if err != nil {
		log.Fatalln(fmt.Errorf("error listening on socket: %s", err.Error()))
	}
	defer listener.Close()

	log.Println("Listening on", c.Socket)

	// Create a new sshizzle agent
	sshizzleAgent := sshizzleagent.NewSSHizzleAgent(c)

	// Catch interrupt/kill signals to exit nicely
	sigs := make(chan os.Signal)
	signal.Notify(sigs, os.Interrupt, os.Kill)
	go func() {
		_ = <-sigs
		err := listener.Close()
		if err != nil {
			log.Fatalln(err.Error())
		}
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			// Catch SIGINT sent to Agent by user
			if strings.Contains(err.Error(), "use of closed network connection") {
				return nil
			}
			log.Fatalln(fmt.Errorf("listener error: %s", err.Error()))
		}
		// Don't return on EOF so socket stays open after serving each request
		if err := agent.ServeAgent(sshizzleAgent, conn); err != nil && err.Error() != "EOF" {
			log.Fatalln(fmt.Errorf("error serving agent on listener: %s", err.Error()))
		}
	}
}
