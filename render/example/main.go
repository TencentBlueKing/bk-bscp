/*
 * Tencent is pleased to support the open source community by making Blueking Container Service available.
 * Copyright (C) 2019 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 * http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing, software distributed under
 * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"encoding/xml"
	"fmt"
	"log"
	"strings"

	"github.com/TencentBlueKing/bk-bscp/render"
)

// nolint
func main() {
	// Create a new renderer with correct script path
	// When running from render/example directory, script is at ../python/main.py
	renderer, err := render.NewRenderer(
		render.WithScriptPath("../python/main.py"),
	)
	if err != nil {
		log.Fatalf("Failed to create renderer: %v", err)
	}

	// Build a small cc topology XML to demonstrate cc / this.* usage
	// Structure: Business(Set->Module->Host)
	ccXML := `<?xml version="1.0" encoding="UTF-8"?>
<Business Name="demo">
  <Set SetName="set-A">
	<Module ModuleName="module-X">
	  <Host InnerIP="10.0.0.1" bk_cloud_id="0" OS="linux" />
	  <Host InnerIP="10.0.0.2" bk_cloud_id="0" OS="linux" />
	</Module>
  </Set>
</Business>`

	// Validate XML just to ensure no syntax error (optional)
	if err = xml.Unmarshal([]byte(ccXML), new(interface{})); err != nil {
		log.Fatalf("invalid ccXML: %v", err)
	}

	// Example 1: Simple template with cc context and this object
	fmt.Println("Example 1: Complete context built in Go")
	template1 := strings.TrimSpace(`Hello ${name}!
Total Hosts: ${len(cc.findall('.//Host'))}

First Host IP:
% if cc.findall('.//Host'):
	${cc.findall('.//Host')[0].get('InnerIP')}
% endif

This object - Set: ${this.set_name}
This object - Module: ${this.module_name}
This object - Custom field: ${this.custom_field}
This attrib: ${this.attrib.get('my_key', 'default')}

Current Host IP via this.cc_host:
% if getattr(this, 'cc_host', None):
	${this.cc_host.get('InnerIP')}
% else:
	N/A
% endif`)

	// Build complete context in Go
	context1 := map[string]interface{}{
		"name":            "BSCP",
		"cc_xml":          ccXML,
		"bk_set_name":     "set-A",
		"bk_module_name":  "module-X",
		"bk_host_innerip": "10.0.0.1",
		"bk_cloud_id":     0,
		// Build 'this' object in Go
		"this": map[string]interface{}{
			"set_name":     "set-A",
			"module_name":  "module-X",
			"custom_field": "my-custom-value",
			"attrib": map[string]interface{}{
				"my_key":  "my-value",
				"another": 123,
			},
			// Can add any new fields
			"new_field": "new data",
		},
	}

	result1, err := renderer.Render(template1, context1)
	if err != nil {
		log.Fatalf("Render failed: %v", err)
	}
	fmt.Printf("Result:\n%s\n\n", result1)

	// Example 2: Template with multiple variables - simple context
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
