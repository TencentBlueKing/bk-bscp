package auth

import (
	"strings"
	"testing"

	pbas "github.com/TencentBlueKing/bk-bscp/pkg/protocol/auth-server"
)

func TestValidateAuthConf(t *testing.T) {
	base := &pbas.GetAuthConfResp{
		LoginAuth: &pbas.LoginAuth{
			Host:     "https://paas.example.com",
			Provider: "bk_login",
		},
		Esb: &pbas.ESB{
			Endpoints: []string{"https://bkapi.example.com"},
			AppCode:   "app_code",
			AppSecret: "app_secret",
			User:      "admin",
		},
		Cmdb: &pbas.CMDB{
			Host:       "https://cmdb.example.com",
			AppCode:    "app_code",
			AppSecret:  "app_secret",
			BkUserName: "admin",
		},
	}

	tests := []struct {
		name    string
		input   *pbas.GetAuthConfResp
		wantErr string
	}{
		{
			name:    "nil response",
			input:   nil,
			wantErr: "response is nil",
		},
		{
			name: "missing cmdb host",
			input: &pbas.GetAuthConfResp{
				LoginAuth: base.LoginAuth,
				Esb:       base.Esb,
				Cmdb: &pbas.CMDB{
					AppCode:    "app_code",
					AppSecret:  "app_secret",
					BkUserName: "admin",
				},
			},
			wantErr: "cmdb.host",
		},
		{
			name: "invalid cmdb host format",
			input: &pbas.GetAuthConfResp{
				LoginAuth: base.LoginAuth,
				Esb:       base.Esb,
				Cmdb: &pbas.CMDB{
					Host:       "cmdb.example.com",
					AppCode:    "app_code",
					AppSecret:  "app_secret",
					BkUserName: "admin",
				},
			},
			wantErr: "invalid cmdb.host",
		},
		{
			name:  "valid",
			input: base,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateAuthConf(tc.input)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("expected nil error, got %v", err)
				}
				return
			}

			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tc.wantErr, err.Error())
			}
		})
	}
}
