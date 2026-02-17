package main

import (
	"log"

	"github.com/typedduck/nestor/controller/executor"
	"github.com/typedduck/nestor/modules"
	"github.com/typedduck/nestor/playbook"
)

func main() {
	// Example 1: Simple deployment using library API
	// simpleDeployment()

	// Example 2: Full deployment with explicit executor configuration
	fullDeploymentWithConfig()

	// Example 3: Initialize a new remote system
	// initializeRemoteSystem()
}

// simpleDeployment shows the simplest way to deploy using nestor
func simpleDeployment() {
	log.Println("[INFO ] simple deployment example")

	// Create a playbook
	pb := playbook.New("simple-webserver")

	// Define what to do
	modules.Package(pb, "update")
	modules.Package(pb, "install", "nginx", "vim", "git")

	modules.File(pb, "/var/www/html/index.html",
		modules.Content(`<!DOCTYPE html>
<html>
<head><title>Welcome</title></head>
<body><h1>Deployed with Nestor!</h1></body>
</html>`))

	modules.File(pb, "/etc/motd",
		modules.Content("Welcome to a nestor-managed server\n"))

	err := executor.Deploy(pb, "user@webserver-01.example.com", &executor.Config{
		SSHKeyPath: "~/.ssh/id_nestor_ed25519",
		DryRun:     true,
	})
	if err != nil {
		log.Printf("[ERROR] %v", err)
		log.Fatalln("[FATAL] deployment failed")
	}
}

// fullDeploymentWithConfig shows how to use explicit configuration
func fullDeploymentWithConfig() {
	log.Println("[INFO ] full deployment with custom config")

	// Create playbook
	pb := playbook.New("webapp-deployment")
	pb.SetEnv("APP_VERSION", "v2.0.0")
	pb.SetEnv("ENVIRONMENT", "production")

	// Install dependencies
	modules.Package(pb, "install", "nginx", "postgresql", "redis-server")

	// Create application directory structure
	modules.Directory(pb, "/opt/webapp",
		modules.Owner("webapp", "webapp"),
		modules.Recursive(true))

	modules.Directory(pb, "/opt/webapp/logs",
		modules.Owner("webapp", "webapp"),
		modules.Mode(0755))

	// Upload application binary
	modules.File(pb, "/opt/webapp/app",
		modules.FromFile("./build/webapp-v2.0.0"),
		modules.Owner("webapp", "webapp"),
		modules.Mode(0755))

	// Deploy configuration from template
	modules.File(pb, "/opt/webapp/config.toml",
		modules.FromTemplate("config.toml.tmpl"),
		modules.TemplateVars(map[string]string{
			"DBHost":    "db.example.com",
			"DBPort":    "5432",
			"DBName":    "webapp_prod",
			"RedisHost": "localhost",
			"RedisPort": "6379",
			"LogLevel":  "info",
			"Port":      "8080",
		}),
		modules.Owner("webapp", "webapp"),
		modules.Mode(0640))

	// Create systemd service
	modules.File(pb, "/etc/systemd/system/webapp.service",
		modules.FromTemplate("webapp.service.tmpl"),
		modules.TemplateVars(map[string]string{
			"WorkingDirectory": "/opt/webapp",
			"ExecStart":        "/opt/webapp/app",
			"User":             "webapp",
			"Group":            "webapp",
		}))

	// Configure nginx
	modules.File(pb, "/etc/nginx/sites-available/webapp",
		modules.FromTemplate("nginx-webapp.conf.tmpl"),
		modules.TemplateVars(map[string]string{
			"ServerName": "webapp.example.com",
			"ProxyPass":  "http://127.0.0.1:8080",
		}))

	modules.Symlink(pb,
		"/etc/nginx/sites-enabled/webapp",
		"/etc/nginx/sites-available/webapp")

	// Deploy the playbook
	err := executor.Deploy(pb, "deploy@webapp-01.example.com", &executor.Config{
		WorkDir:        "/tmp/nestor-work",
		SSHKeyPath:     "/home/user/.ssh/deploy_key",
		SigningKeyPath: "/home/user/.ssh/nestor_signing_key",
		KnownHostsPath: "/home/user/.ssh/known_hosts",
		AgentPath:      "/usr/local/bin/nestor-agent",
		DryRun:         true,
	})
	if err != nil {
		log.Printf("[ERROR] %v", err)
		log.Fatalln("[FATAL] deployment failed")
	}
}

// initializeRemoteSystem shows how to initialize a remote system for nestor
func initializeRemoteSystem() {
	// Initialize the remote system
	// This will:
	// 1. Transfer the nestor-agent binary
	// 2. Install it with appropriate permissions
	// 3. Add the controller's public key to authorized_keys
	if err := executor.InitRemote("user@newserver.example.com", "./build/nestor-agent",
		&executor.Config{}); err != nil {
		log.Printf("[ERROR] %v", err)
		log.Fatalln("[FATAL] initialization failed")
	}
}
