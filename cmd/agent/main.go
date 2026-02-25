package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/typedduck/nestor/agent/executor"
	"github.com/typedduck/nestor/agent/handlers"
	"github.com/typedduck/nestor/agent/validator"
)

const (
	Version   = "dev"
	AgentName = "nestor-agent"
)

// AgentConfig holds the configuration for the agent
type AgentConfig struct {
	PlaybookPath  string
	StateFile     string
	DetachOnError bool
	LogFile       string
	DryRun        bool
}

func main() {
	// Parse command line flags
	config := parseFlags()

	// Setup logging
	if err := setupLogging(config.LogFile); err != nil {
		log.Fatalf("[FATAL] failed to setup logging: %v", err)
	}

	log.Printf("[INFO ] %s version %s starting...", AgentName, Version)

	// Check if running as root
	if !executor.IsRoot() {
		log.Fatal("[FATAL] agent must be run as root (use sudo)")
	}

	// Derive the extraction path once so both loadPlaybook and the validator
	// reference the same directory.
	extractPath := "/tmp/nestor-playbook-" + fmt.Sprintf("%d", time.Now().Unix())

	// Load and validate the playbook
	playbook, err := loadPlaybook(config.PlaybookPath, extractPath)
	if err != nil {
		log.Fatalf("[FATAL] failed to load playbook: %v", err)
	}

	log.Printf("[INFO ] loaded playbook: %s (version %s)", playbook.Name, playbook.Version)
	log.Printf("[INFO ] created by: %s at %s", playbook.Controller, playbook.Created)
	log.Printf("[INFO ] actions to execute: %d", len(playbook.Actions))

	// Create platform implementations
	fs := executor.OSFileSystem{}
	cmd := executor.OSCommandRunner{}

	// Validate playbook integrity
	val := validator.New(config.PlaybookPath, extractPath, nil)
	if err := val.ValidateSignature(); err != nil {
		log.Fatalf("[FATAL] signature validation failed: %v", err)
	}
	if err := val.ValidateManifest(); err != nil {
		log.Fatalf("[FATAL] Manifest validation failed: %v", err)
	}

	log.Println("[INFO ] playbook validation successful")

	// Detect executor capabilities
	sysInfo := executor.DetectSystem(nil, nil)
	log.Printf("[INFO ] executor detected: OS=%s, PackageManager=%s, InitSystem=%s",
		sysInfo.OS, sysInfo.PackageManager, sysInfo.InitSystem)

	// Create execution engine
	engine := executor.New(playbook, sysInfo, config.StateFile, fs, cmd)
	engine.SetDryRun(config.DryRun)

	// Register action handlers
	registerHandlers(engine)

	// Execute the playbook
	log.Println("[INFO] starting playbook execution...")
	result, err := engine.Execute()
	if err != nil {
		log.Fatalf("[FATAL] execution failed: %v", err)
	}

	// Display results
	displayResults(result)

	// Exit with appropriate code
	if result.Summary.Failed > 0 {
		os.Exit(1)
	}
}

// parseFlags parses command line flags
func parseFlags() *AgentConfig {
	config := &AgentConfig{}

	flag.StringVar(&config.PlaybookPath, "playbook", "/tmp/playbook.tar.gz",
		"Path to the playbook archive")
	flag.StringVar(&config.StateFile, "state", "/tmp/nestor-agent.state",
		"Path to the state file for detach/reattach")
	flag.BoolVar(&config.DetachOnError, "detach-on-error", false,
		"Detach on SSH connection error")
	flag.StringVar(&config.LogFile, "log", "",
		"Log file path (default: stderr)")
	flag.BoolVar(&config.DryRun, "dry-run", false,
		"Show what would be done without making changes")

	flag.Parse()

	return config
}

// setupLogging configures the logging executor
func setupLogging(logFile string) error {
	if logFile != "" {
		f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return err
		}
		log.SetOutput(f)
	}

	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	return nil
}

// loadPlaybook extracts the archive to extractPath and unmarshals playbook.json.
func loadPlaybook(path, extractPath string) (*executor.Playbook, error) {
	if err := extractArchive(path, extractPath); err != nil {
		return nil, fmt.Errorf("failed to extract archive: %w", err)
	}

	data, err := os.ReadFile(filepath.Join(extractPath, "playbook.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to read playbook.json: %w", err)
	}

	var playbook executor.Playbook
	if err := json.Unmarshal(data, &playbook); err != nil {
		return nil, fmt.Errorf("failed to unmarshal playbook: %w", err)
	}

	playbook.ExtractPath = extractPath

	return &playbook, nil
}

// extractArchive extracts a tar.gz archive into extractPath.
// It guards against path-traversal attacks by rejecting any entry whose
// resolved destination does not reside under extractPath.
func extractArchive(archivePath, extractPath string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := os.MkdirAll(extractPath, 0755); err != nil {
		return err
	}

	gr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar: %w", err)
		}

		// Guard against path traversal.
		target := filepath.Join(extractPath, hdr.Name)
		if !strings.HasPrefix(target, extractPath+string(os.PathSeparator)) &&
			target != extractPath {
			return fmt.Errorf("tar entry %q escapes extraction directory", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(hdr.Mode)); err != nil {
				return err
			}

		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC,
				os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		}
	}

	return nil
}

// registerHandlers registers all action handlers with the execution engine
func registerHandlers(engine *executor.Engine) {
	// Package handlers
	engine.RegisterHandler("package.install", handlers.NewPackageInstallHandler())
	engine.RegisterHandler("package.remove", handlers.NewPackageRemoveHandler())
	engine.RegisterHandler("package.update", handlers.NewPackageUpdateHandler())
	engine.RegisterHandler("package.upgrade", handlers.NewPackageUpgradeHandler())

	// File handlers
	engine.RegisterHandler("file.content", handlers.NewFileContentHandler())
	engine.RegisterHandler("file.template", handlers.NewFileTemplateHandler())
	engine.RegisterHandler("file.upload", handlers.NewFileUploadHandler())
	engine.RegisterHandler("file.symlink", handlers.NewSymlinkHandler())

	// Directory handlers
	engine.RegisterHandler("directory.create", handlers.NewDirectoryCreateHandler())

	// File removal handlers
	engine.RegisterHandler("file.remove", handlers.NewFileRemoveHandler())

	// Service handlers
	engine.RegisterHandler("service.start", handlers.NewServiceStartHandler())
	engine.RegisterHandler("service.stop", handlers.NewServiceStopHandler())
	engine.RegisterHandler("service.reload", handlers.NewServiceReloadHandler())
	engine.RegisterHandler("service.restart", handlers.NewServiceRestartHandler())

	// Command handlers (to be implemented)
	// engine.RegisterHandler("command.execute", handlers.NewCommandExecuteHandler())
	// engine.RegisterHandler("script.execute", handlers.NewScriptExecuteHandler())
}

// displayResults displays the execution results
func displayResults(result *executor.ExecutionResult) {
	log.Printf("[INFO ] playbook: %s", result.PlaybookID)
	log.Printf("[INFO ] status: %s", result.Status)
	log.Printf("[INFO ] started: %s", result.Started.Format(time.RFC3339))
	log.Printf("[INFO ] completed: %s", result.Completed.Format(time.RFC3339))
	log.Printf("[INFO ] duration: %.2f seconds", result.DurationSeconds)
	log.Println()
	log.Printf("[INFO ] total actions: %d", result.Summary.Total)
	log.Printf("[INFO ]   success: %d", result.Summary.Success)
	log.Printf("[INFO ]   failed: %d", result.Summary.Failed)
	log.Printf("[INFO ]   skipped: %d", result.Summary.Skipped)
	log.Printf("[INFO ]   changed: %d", result.Summary.Changed)

	if result.Summary.Failed > 0 {
		log.Printf("[ERROR] %d actions failed:", result.Summary.Failed)
		for _, action := range result.Actions {
			if action.Status == "failed" {
				log.Printf("[ERROR]   [%s] %s: %s", action.ID, action.Type, action.Message)
			}
		}
	}
}
