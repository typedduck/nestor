package main

import (
	"fmt"
	"log"

	"github.com/typedduck/nestor/modules"
	"github.com/typedduck/nestor/playbook"
)

func main() {
	// Create a new playbook for server setup
	pb := playbook.New("webserver-setup")

	// Set environment variables
	pb.SetEnv("ENVIRONMENT", "production")
	pb.SetEnv("SERVER_ROLE", "webserver")

	// Update package cache first
	if err := modules.Package(pb, "update"); err != nil {
		log.Fatalf("Failed to add package update: %v", err)
	}

	// Install web server and utilities
	if err := modules.Package(pb, "install", "nginx", "vim", "git", "htop"); err != nil {
		log.Fatalf("Failed to add package install: %v", err)
	}

	// Remove unnecessary packages
	if err := modules.Package(pb, "remove", "apache2"); err != nil {
		log.Fatalf("Failed to add package remove: %v", err)
	}

	// Upgrade all packages to latest versions
	if err := modules.Package(pb, "upgrade"); err != nil {
		log.Fatalf("Failed to add package upgrade: %v", err)
	}

	// Display the generated playbook
	fmt.Println("Generated Playbook:")
	fmt.Println("===================")
	jsonData, err := pb.ToJSON()
	if err != nil {
		log.Fatalf("Failed to serialize playbook: %v", err)
	}
	fmt.Println(string(jsonData))

	// Execute on remote host
	// In real usage, this would package and transfer the playbook
	fmt.Println("\nExecuting playbook...")
	// if err := pb.Execute("user@webserver-01.example.com"); err != nil {
	// 	log.Fatalf("Failed to execute playbook: %v", err)
	// }
}
