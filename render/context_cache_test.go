package render

import (
	"context"
	"testing"

	"github.com/TencentBlueKing/bk-bscp/internal/components/bkcmdb"
	processorcmdb "github.com/TencentBlueKing/bk-bscp/internal/processor/cmdb"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

type panicCMDBService struct {
	bkcmdb.Service
}

func (p *panicCMDBService) FindTopoBrief(_ context.Context, _ int) (*bkcmdb.TopoBriefResp, error) {
	panic("FindTopoBrief should not be called when render cache hits")
}

func (p *panicCMDBService) SearchObjectAttr(_ context.Context, _ bkcmdb.SearchObjectAttrReq) ([]bkcmdb.ObjectAttrInfo, error) {
	panic("SearchObjectAttr should not be called when render cache hits")
}

type testProcessSource struct {
	process *table.Process
}

func (s *testProcessSource) GetProcess() *table.Process {
	return s.process
}

func (s *testProcessSource) GetProcessInstance() *table.ProcessInstance {
	return &table.ProcessInstance{
		Spec: &table.ProcessInstanceSpec{
			HostInstSeq:   1,
			ModuleInstSeq: 2,
		},
	}
}

func (s *testProcessSource) GetModuleInstSeq() uint32 {
	return 0
}

func (s *testProcessSource) NeedHelp() bool {
	return false
}

func TestBuildProcessContextParamsFromSourceUsesRenderCache(t *testing.T) {
	const (
		tenantID = "tenant-a"
		bizID    = 42
		setEnv   = "3"
		ccXML    = `<?xml version="1.0" encoding="UTF-8"?><Application></Application>`
	)
	cache := processorcmdb.NewMemoryCMDBRenderCache()
	cache.SetTopoXML(context.Background(), tenantID, bizID, setEnv, ccXML)
	cache.SetBizObjectAttributes(context.Background(), tenantID, bizID, map[string][]processorcmdb.ObjectAttribute{
		processorcmdb.BK_SET_OBJ_ID: {{BkPropertyID: "bk_set_name"}},
	})

	kt := kit.NewWithTenant(tenantID)
	params := BuildProcessContextParamsFromSource(kt.Ctx, &testProcessSource{
		process: &table.Process{
			Spec: &table.ProcessSpec{
				SetName:     "set-a",
				ModuleName:  "module-a",
				ServiceName: "svc-a",
				Environment: setEnv,
			},
			Attachment: &table.ProcessAttachment{
				TenantID:    tenantID,
				BizID:       bizID,
				CcProcessID: 100,
			},
		},
	}, &panicCMDBService{}, cache)

	if params.CcXML != ccXML {
		t.Fatalf("CcXML = %q, want cached xml", params.CcXML)
	}
	if _, ok := params.GlobalVariables["biz_global_variables"]; !ok {
		t.Fatal("biz_global_variables should be populated from render cache")
	}
}
