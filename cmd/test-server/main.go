package main

import (
	"log"
	"os"

	"github.com/GoogleCloudPlatform/functions-framework-go/funcframework"
	_ "github.com/tsaarni/dyndns" // Blank import to register the function.
)

func main() {
	// Use PORT environment variable, or default to 8080.
	port := "8080"
	if envPort := os.Getenv("PORT"); envPort != "" {
		port = envPort
	}

	if err := funcframework.StartHostPort("127.0.0.1", port); err != nil {
		log.Fatalf("funcframework.StartHostPort: %v\n", err)
	}

}
