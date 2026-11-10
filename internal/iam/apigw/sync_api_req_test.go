package apigw

import (
	"encoding/json"
	"testing"
)

func TestSyncApiReqOfficialType(t *testing.T) {
	body, err := json.Marshal(SyncApiReq{
		Description: "服务配置中心",
		Maintainers: []string{"admin"},
		IsPublic:    true,
		ApiType:     apiTypeOfficial,
	})
	if err != nil {
		t.Fatalf("marshal sync api request: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal sync api request: %v", err)
	}
	apiType, ok := got["api_type"].(float64)
	if !ok || apiType != apiTypeOfficial {
		t.Fatalf("api_type = %#v, want %d", got["api_type"], apiTypeOfficial)
	}
}
