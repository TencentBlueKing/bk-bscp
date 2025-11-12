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

package render_test

import (
	"context"
	"testing"
	"time"

	"github.com/TencentBlueKing/bk-bscp/render"
)

func TestRenderer_Render(t *testing.T) {
	renderer, err := render.NewRenderer()
	if err != nil {
		t.Fatalf("Failed to create renderer: %v", err)
	}

	tests := []struct {
		name     string
		template string
		context  map[string]interface{}
		want     string
		wantErr  bool
	}{
		{
			name:     "simple template",
			template: "Hello ${name}!",
			context: map[string]interface{}{
				"name": "World",
			},
			want:    "Hello World!",
			wantErr: false,
		},
		{
			name:     "template with multiple variables",
			template: "Server: ${server}\nPort: ${port}",
			context: map[string]interface{}{
				"server": "bk-bscp",
				"port":   8080,
			},
			want:    "Server: bk-bscp\nPort: 8080",
			wantErr: false,
		},
		{
			name:     "empty template",
			template: "",
			context:  map[string]interface{}{},
			want:     "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := renderer.Render(tt.template, tt.context)
			if (err != nil) != tt.wantErr {
				t.Errorf("Render() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("Render() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRenderer_RenderWithContext(t *testing.T) {
	renderer, err := render.NewRenderer()
	if err != nil {
		t.Fatalf("Failed to create renderer: %v", err)
	}

	t.Run("with timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		template := "Hello ${name}!"
		context := map[string]interface{}{
			"name": "BSCP",
		}

		got, err := renderer.RenderWithContext(ctx, template, context)
		if err != nil {
			t.Errorf("RenderWithContext() error = %v", err)
			return
		}

		want := "Hello BSCP!"
		if got != want {
			t.Errorf("RenderWithContext() = %v, want %v", got, want)
		}
	})
}

func TestRenderer_RenderWithTempFile(t *testing.T) {
	renderer, err := render.NewRenderer()
	if err != nil {
		t.Fatalf("Failed to create renderer: %v", err)
	}

	t.Run("large context", func(t *testing.T) {
		template := "Count: ${count}\nData: ${data}"
		context := map[string]interface{}{
			"count": 1000,
			"data":  "This is a large data context that should be passed via file",
		}

		got, err := renderer.RenderWithTempFile(template, context)
		if err != nil {
			t.Errorf("RenderWithTempFile() error = %v", err)
			return
		}

		if got == "" {
			t.Errorf("RenderWithTempFile() returned empty result")
		}
	})
}
