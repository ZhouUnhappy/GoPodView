package main

import (
	"flag"
	"fmt"
	"log"

	"gopodview/internal/api"
)

func main() {
	projectPath := flag.String("project", "", "path to the Go project to analyze")
	port := flag.Int("port", 8080, "HTTP server port")
	frontendPort := flag.Int("frontend-port", 5173, "Frontend dev server port for CORS")
	flag.Parse()

	handler := api.NewHandler(*projectPath)
	router := api.SetupRouter(handler, *frontendPort)

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("GoPodView backend starting on %s", addr)
	if *projectPath != "" {
		log.Printf("Analyzing project: %s", *projectPath)
	} else {
		log.Printf("No project loaded. Use POST /api/project to set one.")
	}

	if err := router.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
