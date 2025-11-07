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
