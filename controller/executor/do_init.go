package executor

import (
	"fmt"
	"log"

	"github.com/typedduck/nestor/controller/signer"
	"github.com/typedduck/nestor/util"
)

// InitRemote initializes a remote system for use with nestor
//
// This method:
// 1. Transfers the agent binary to the remote system
// 2. Installs the agent with appropriate permissions
// 3. Adds the controller's public key to authorized_keys (for signature verification)
func (e *Executor) InitRemote(host, agentBinaryPath string) error {
	log.Printf("[INFO ] initializing %s for nestor", host)

	// Parse host
	user, hostname, port, err := util.ParseHost(host)
	if err != nil {
		return fmt.Errorf("invalid host format: %w", err)
	}

	// Connect to remote host
	log.Printf("[INFO ] connecting to %s...", host)
	client, err := e.connectSSH(user, hostname, port)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer client.Close()

	// Resolve {os}/{arch} placeholders in the agent binary path.
	if HasArchPlaceholders(agentBinaryPath) {
		log.Println("[INFO ] detecting remote system architecture...")
		goos, goarch, err := detectRemoteSystem(client)
		if err != nil {
			return fmt.Errorf("failed to detect remote system: %w", err)
		}
		agentBinaryPath = ResolveAgentPath(agentBinaryPath, goos, goarch)
		log.Printf("[INFO ] resolved agent binary: %s", agentBinaryPath)
	}

	// Install agent
	log.Println("[INFO ] installing nestor-agent...")
	if err := client.InstallAgent(agentBinaryPath); err != nil {
		return fmt.Errorf("failed to install agent: %w", err)
	}

	// Get controller's public key
	sgn, err := signer.New(e.signingKeyPath)
	if err != nil {
		return fmt.Errorf("failed to load signing key: %w", err)
	}

	publicKey, err := sgn.GetPublicKey()
	if err != nil {
		return fmt.Errorf("failed to get public key: %w", err)
	}

	// Add public key to authorized_keys
	log.Println("[INFO ] adding controller's public key...")
	if err := client.AddAuthorizedKey(publicKey); err != nil {
		return fmt.Errorf("failed to add authorized key: %w", err)
	}

	log.Println("[INFO ] initialization complete")
	log.Printf("[INFO ] agent installed at: %s", e.agentPath)
	log.Println("[INFO ] controller public key added to authorized_keys")

	return nil
}
