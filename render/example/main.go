package main

import (
	"fmt"
	"log"

	"github.com/TencentBlueKing/bk-bscp/render"
)

func main() {
	// Create a new renderer with correct script path
	// When running from render/example directory, script is at ../python/main.py
	renderer, err := render.NewRenderer(
		render.WithScriptPath("../python/main.py"),
	)
	if err != nil {
		log.Fatalf("Failed to create renderer: %v", err)
	}

	// Example 1: Simple template rendering
	fmt.Println("Example 1: Simple template")
	template1 := "Hello ${name}!"
	context1 := map[string]interface{}{
		"name": "BSCP",
	}
	result1, err := renderer.Render(template1, context1)
	if err != nil {
		log.Fatalf("Render failed: %v", err)
	}
	fmt.Printf("Result: %s\n\n", result1)

	// Example 2: Template with multiple variables
	fmt.Println("Example 2: Multiple variables")
	template2 := `Server Configuration:
Name: ${server_name}
Port: ${port}
Environment: ${environment}`
	context2 := map[string]interface{}{
		"server_name": "bk-bscp-server",
		"port":        8080,
		"environment": "production",
	}
	result2, err := renderer.Render(template2, context2)
	if err != nil {
		log.Fatalf("Render failed: %v", err)
	}
	fmt.Printf("Result:\n%s\n\n", result2)

	// Example 3: Template with conditional logic
	fmt.Println("Example 3: Conditional logic")
	template3 := `Status: ${status}
% if status == "active":
Service is running
% else:
Service is stopped
% endif`
	context3 := map[string]interface{}{
		"status": "active",
	}
	result3, err := renderer.Render(template3, context3)
	if err != nil {
		log.Fatalf("Render failed: %v", err)
	}
	fmt.Printf("Result:\n%s\n\n", result3)

	// Example 4: Using temp file for large context
	fmt.Println("Example 4: Large context with temp file")
	template4 := "Process: ${process_name}\nID: ${process_id}"
	context4 := map[string]interface{}{
		"process_name": "bk-bscp-apiserver",
		"process_id":   12345,
	}
	result4, err := renderer.RenderWithTempFile(template4, context4)
	if err != nil {
		log.Fatalf("RenderWithTempFile failed: %v", err)
	}
	fmt.Printf("Result:\n%s\n", result4)
}
