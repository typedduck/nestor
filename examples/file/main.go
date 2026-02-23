package main

import (
	"fmt"
	"log"

	"github.com/typedduck/nestor/modules"
	"github.com/typedduck/nestor/playbook/builder"
)

func main() {
	// Create a new playbook for deploying a web application
	b := builder.New("webapp-file-deployment")

	// Set environment variables
	b.SetEnv("ENVIRONMENT", "production")
	b.SetEnv("APP_VERSION", "v2.1.0")

	// 1. Create application directory structure
	fmt.Println("Setting up directory structure...")
	if err := modules.Directory(b, "/opt/webapp",
		modules.Owner("webapp", "webapp"),
		modules.Mode(0755)); err != nil {
		log.Fatalf("Failed to create webapp directory: %v", err)
	}

	if err := modules.Directory(b, "/opt/webapp/config",
		modules.Owner("webapp", "webapp"),
		modules.Mode(0750),
		modules.Recursive(true)); err != nil {
		log.Fatalf("Failed to create config directory: %v", err)
	}

	if err := modules.Directory(b, "/opt/webapp/logs",
		modules.Owner("webapp", "webapp"),
		modules.Mode(0755),
		modules.Recursive(true)); err != nil {
		log.Fatalf("Failed to create logs directory: %v", err)
	}

	// 2. Upload application binary
	fmt.Println("Uploading application binary...")
	if err := modules.File(b, "/opt/webapp/webapp",
		modules.FromFile("./build/webapp-v2.1.0"),
		modules.Owner("webapp", "webapp"),
		modules.Mode(0755)); err != nil {
		log.Fatalf("Failed to upload webapp binary: %v", err)
	}

	// 3. Create configuration from template
	fmt.Println("Creating configuration files...")
	if err := modules.File(b, "/opt/webapp/config/app.toml",
		modules.FromTemplate("app.toml.tmpl"),
		modules.TemplateVars(map[string]string{
			"Environment": "production",
			"DBHost":      "db.example.com",
			"DBPort":      "5432",
			"DBName":      "webapp_prod",
			"LogLevel":    "info",
			"Port":        "8080",
		}),
		modules.Owner("webapp", "webapp"),
		modules.Mode(0640)); err != nil {
		log.Fatalf("Failed to create config file: %v", err)
	}

	// 4. Create systemd service file
	fmt.Println("Creating systemd service...")
	if err := modules.File(b, "/etc/systemd/system/webapp.service",
		modules.FromTemplate("webapp.service.tmpl"),
		modules.TemplateVars(map[string]string{
			"WorkingDirectory": "/opt/webapp",
			"ExecStart":        "/opt/webapp/webapp",
			"User":             "webapp",
			"Group":            "webapp",
		}),
		modules.Mode(0644)); err != nil {
		log.Fatalf("Failed to create systemd service: %v", err)
	}

	// 5. Create nginx configuration
	fmt.Println("Configuring nginx...")
	if err := modules.File(b, "/etc/nginx/sites-available/webapp",
		modules.FromTemplate("nginx-webapp.conf.tmpl"),
		modules.TemplateVars(map[string]string{
			"ServerName": "webapp.example.com",
			"ProxyPass":  "http://127.0.0.1:8080",
			"AccessLog":  "/var/log/nginx/webapp-access.log",
			"ErrorLog":   "/var/log/nginx/webapp-error.log",
		}),
		modules.Mode(0644)); err != nil {
		log.Fatalf("Failed to create nginx config: %v", err)
	}

	// 6. Enable nginx site with symlink
	if err := modules.Symlink(b,
		"/etc/nginx/sites-enabled/webapp",
		"/etc/nginx/sites-available/webapp"); err != nil {
		log.Fatalf("Failed to create nginx symlink: %v", err)
	}

	// 7. Create a simple static HTML file
	fmt.Println("Creating static content...")
	if err := modules.File(b, "/opt/webapp/public/index.html",
		modules.Content(`<!DOCTYPE html>
<html>
<head>
    <title>WebApp v2.1.0</title>
</head>
<body>
    <h1>Welcome to WebApp</h1>
    <p>Version: 2.1.0</p>
    <p>Environment: Production</p>
</body>
</html>`),
		modules.Owner("webapp", "webapp"),
		modules.Mode(0644)); err != nil {
		log.Fatalf("Failed to create HTML file: %v", err)
	}

	// 8. Create environment file with secrets
	if err := modules.File(b, "/opt/webapp/.env",
		modules.Content("DB_PASSWORD=super_secret_password\nAPI_KEY=abc123def456"),
		modules.Owner("webapp", "webapp"),
		modules.Mode(0600)); err != nil {
		log.Fatalf("Failed to create env file: %v", err)
	}

	// 9. Remove old version if it exists
	fmt.Println("Cleaning up old versions...")
	if err := modules.Remove(b, "/opt/webapp-old",
		modules.Recursive(true)); err != nil {
		log.Fatalf("Failed to remove old version: %v", err)
	}

	// Display the generated playbook
	fmt.Println("\n" + string([]rune{0x2500}) + " Generated Playbook " + string([]rune{0x2500, 0x2500, 0x2500, 0x2500, 0x2500, 0x2500, 0x2500, 0x2500, 0x2500, 0x2500, 0x2500, 0x2500, 0x2500}))
	jsonData, err := b.Playbook().ToJSON()
	if err != nil {
		log.Fatalf("Failed to serialize playbook: %v", err)
	}
	fmt.Println(string(jsonData))

	// Execute on remote host
	fmt.Println("\n" + string([]rune{0x2500}) + " Executing Playbook " + string([]rune{0x2500, 0x2500, 0x2500, 0x2500, 0x2500, 0x2500, 0x2500, 0x2500, 0x2500, 0x2500, 0x2500, 0x2500, 0x2500}))
	// if err := pb.Execute("deploy@webapp-01.example.com"); err != nil {
	// 	log.Fatalf("Failed to execute playbook: %v", err)
	// }

	fmt.Println("\n✓ Deployment playbook generated successfully!")
	fmt.Printf("  - Created %d file operations\n", len(b.Playbook().Actions))
	fmt.Println("  - Ready to deploy to: deploy@webapp-01.example.com")
}
