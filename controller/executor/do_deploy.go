package executor

import (
	"fmt"
	"log"
	"path"
	"path/filepath"
	"time"

	"github.com/typedduck/nestor/controller/packager"
	"github.com/typedduck/nestor/controller/signer"
	"github.com/typedduck/nestor/controller/ssh"
	"github.com/typedduck/nestor/playbook"
)

// Deploy executes a deployment (pre-phase, remote, post-phase) on a remote host.
//
// This method orchestrates the complete execution flow:
// 1. Reject --dry-run when pre: or post: phases are present
// 2. Execute pre: phase locally on the controller (if present)
// 3. Package the remote playbook into a tar.gz archive
// 4. Sign the archive with the controller's private key
// 5. Connect to the remote host via SSH
// 6. Transfer the playbook archive
// 7. Deploy the agent on the remote host
// 8. Collect and display results
// 9. Execute post: phase locally on the controller (if present)
func (e *Executor) Deploy(d *Deployment, host string) error {
	// Reject --dry-run when pre: or post: phases are present, because those
	// phases have real side-effects that cannot be undone and dry-run mode
	// would give a misleading picture of execution.
	if e.dryRun && (d.Pre != nil || d.Post != nil) {
		return fmt.Errorf("--dry-run is not supported for playbooks with pre: or post: sections")
	}

	log.Printf("[INFO ] executing playbook '%s' on %s", d.Remote.Name, host)

	// Parse host (user@hostname)
	user, hostname, port, err := parseHost(host)
	if err != nil {
		return fmt.Errorf("invalid host format: %w", err)
	}

	// Execute pre: phase on the controller.
	if d.Pre != nil {
		log.Println("[INFO ] executing pre: phase on controller...")
		if err := Local(d.Pre, d.PlaybookDir, false); err != nil {
			return fmt.Errorf("pre: phase failed: %w", err)
		}
	}

	// Step 1: Package the playbook
	log.Println("[INFO ] packaging playbook...")
	pkg, err := e.packagePlaybook(d.Remote)
	if err != nil {
		return fmt.Errorf("failed to package playbook: %w", err)
	}

	// Step 2: Sign the playbook
	log.Println("[INFO ] signing playbook...")
	if err := e.signPlaybook(pkg); err != nil {
		return fmt.Errorf("failed to sign playbook: %w", err)
	}

	// Dry run mode - stop here (only reached when pre/post are nil)
	if e.dryRun {
		log.Println("[INFO ] dry run complete, playbook packaged and signed")
		log.Printf("[INFO ] archive: %s", pkg.ArchivePath)
		return nil
	}

	// Step 3: Connect to remote host
	log.Printf("[INFO ] connecting to %s...", host)
	client, err := e.connectSSH(user, hostname, port)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer client.Close()

	// Step 4: Transfer playbook
	log.Println("[INFO ] transferring playbook...")
	remotePath, err := e.transferPlaybook(client, pkg)
	if err != nil {
		return fmt.Errorf("failed to transfer playbook: %w", err)
	}

	// Step 5: Execute agent
	log.Println("[INFO ] executing agent on remote host...")

	startTime := time.Now()
	_, message, err := client.ExecuteAgent(remotePath, false)

	if err != nil {
		return fmt.Errorf("failed to start agent: %w", err)
	}

	log.Printf("[INFO ] agent started: %s\n", message)

	// Get log and state file paths
	logPath := client.GetLogPath(remotePath)
	statePath := client.GetStateFilePath(remotePath)

	log.Printf("[INFO] log file path: %s", logPath)
	log.Printf("[INFO] state file path: %s", statePath)

	// Follow execution by tailing the log file
	log.Println("======== agent output ========")

	// Tail log in a goroutine so we can monitor agent status
	done := make(chan error)
	go func() {
		done <- client.TailLogFile(logPath, true)
	}()

	// Poll agent status
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Check if agent is still running
			running, err := client.IsAgentRunning()
			if err != nil {
				log.Printf("[WARN ] failed to check agent status: %v", err)
				continue
			}

			if !running {
				// Agent finished, give tail a moment to catch up
				time.Sleep(500 * time.Millisecond)
				goto agentComplete
			}

		case err := <-done:
			// Tail exited (probably because connection dropped)
			if err != nil {
				log.Printf("[WARN ] log streaming stopped: %v", err)
			}
			goto agentComplete
		}
	}

agentComplete:
	duration := time.Since(startTime)

	// Retrieve final state
	log.Println("======== retrieving final results ========")

	result, err := e.Attach(host, statePath)
	if err != nil {
		log.Printf("[WARN ] failed to retrieve results: %v", err)
		log.Println("[INFO ] agent execution completed")
		log.Printf("[INFO ] duration: %.2f seconds", duration.Seconds())
		return nil
	}

	log.Println()
	log.Println("======== execution complete ========")
	log.Printf("[INFO ] playbook: %s", result.PlaybookID)
	log.Printf("[INFO ] status: %s", result.Status)
	log.Printf("[INFO ] duration: %.2f seconds", duration.Seconds())
	log.Printf("[INFO ] total: %d, success: %d, failed: %d, changed: %d",
		result.Summary.Total,
		result.Summary.Success,
		result.Summary.Failed,
		result.Summary.Changed)

	if result.Summary.Failed > 0 {
		return fmt.Errorf("%d actions failed", result.Summary.Failed)
	}

	log.Println("[INFO ] playbook executed successfully")

	// Execute post: phase on the controller.
	if d.Post != nil {
		log.Println("[INFO ] executing post: phase on controller...")
		if err := Local(d.Post, d.PlaybookDir, false); err != nil {
			return fmt.Errorf("post: phase failed: %w", err)
		}
	}

	return nil
}

// packagePlaybook packages the playbook into a tar.gz archive
func (e *Executor) packagePlaybook(pb *playbook.Playbook) (*packager.Package, error) {
	pkgr := packager.New(e.workDir)
	return pkgr.Package(pb)
}

// signPlaybook signs the playbook archive
func (e *Executor) signPlaybook(pkg *packager.Package) error {
	sgn, err := signer.New(e.signingKeyPath)
	if err != nil {
		return err
	}

	return sgn.Sign(pkg)
}

// transferPlaybook transfers the playbook archive to the remote host
func (e *Executor) transferPlaybook(client *ssh.Client, pkg *packager.Package) (string, error) {
	// Transfer to /tmp on remote host
	remotePath := path.Join("/tmp", filepath.ToSlash(pkg.ArchivePath[len(e.workDir)+1:]))

	if err := client.TransferFile(pkg.ArchivePath, remotePath); err != nil {
		return "", err
	}

	return remotePath, nil
}
