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

package bkcmdb

import (
	"context"
	"strings"
	"testing"

	esbbklogin "github.com/TencentBlueKing/bk-bscp/internal/thirdparty/esb/bklogin"
	esbclient "github.com/TencentBlueKing/bk-bscp/internal/thirdparty/esb/client"
	esbcmdb "github.com/TencentBlueKing/bk-bscp/internal/thirdparty/esb/cmdb"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
)

type fakeESBClient struct {
	cmdb esbcmdb.Client
}

func (f *fakeESBClient) Cmdb() esbcmdb.Client {
	return f.cmdb
}

func (f *fakeESBClient) BKLogin() esbbklogin.Client {
	return nil
}

var _ esbclient.Client = (*fakeESBClient)(nil)

type fakeESBCMDBClient struct {
	searchCalled int
	listCalled   int
	searchParams *esbcmdb.SearchBizParams
	searchResp   *esbcmdb.SearchBizResult
	listResp     *esbcmdb.SearchBizResult
}

func (f *fakeESBCMDBClient) SearchBusiness(_ context.Context, params *esbcmdb.SearchBizParams) (
	*esbcmdb.SearchBizResult, error) {

	f.searchCalled++
	f.searchParams = params
	return f.searchResp, nil
}

func (f *fakeESBCMDBClient) ListAllBusiness(_ context.Context) (*esbcmdb.SearchBizResult, error) {
	f.listCalled++
	return f.listResp, nil
}

func (f *fakeESBCMDBClient) GeBusinessByID(_ context.Context, _ uint32) (*esbcmdb.Biz, error) {
	return nil, nil
}

var _ esbcmdb.Client = (*fakeESBCMDBClient)(nil)

func TestSearchBusinessUsesESBWhenConfigured(t *testing.T) {
	expected := &esbcmdb.SearchBizResult{
		Count: 1,
		Info: []esbcmdb.Biz{{
			BizID:   100,
			BizName: "from-esb",
		}},
	}
	esbCMDB := &fakeESBCMDBClient{searchResp: expected}
	svc, err := New(&cc.CMDBConfig{UseEsb: true}, &fakeESBClient{cmdb: esbCMDB})
	if err != nil {
		t.Fatalf("new cmdb service failed: %v", err)
	}

	params := &esbcmdb.SearchBizParams{Fields: []string{"bk_biz_id", "bk_biz_name"}}
	actual, err := svc.SearchBusiness(context.Background(), params)
	if err != nil {
		t.Fatalf("search business failed: %v", err)
	}

	if actual != expected {
		t.Fatalf("expected esb search response, got %#v", actual)
	}
	if esbCMDB.searchCalled != 1 {
		t.Fatalf("expected esb SearchBusiness to be called once, got %d", esbCMDB.searchCalled)
	}
	if esbCMDB.searchParams != params {
		t.Fatalf("expected original search params to be passed to esb")
	}
}

func TestListAllBusinessUsesESBWhenConfigured(t *testing.T) {
	expected := &esbcmdb.SearchBizResult{
		Count: 1,
		Info: []esbcmdb.Biz{{
			BizID:   101,
			BizName: "list-from-esb",
		}},
	}
	esbCMDB := &fakeESBCMDBClient{listResp: expected}
	svc, err := New(&cc.CMDBConfig{UseEsb: true}, &fakeESBClient{cmdb: esbCMDB})
	if err != nil {
		t.Fatalf("new cmdb service failed: %v", err)
	}

	actual, err := svc.ListAllBusiness(context.Background())
	if err != nil {
		t.Fatalf("list all business failed: %v", err)
	}

	if actual != expected {
		t.Fatalf("expected esb list response, got %#v", actual)
	}
	if esbCMDB.listCalled != 1 {
		t.Fatalf("expected esb ListAllBusiness to be called once, got %d", esbCMDB.listCalled)
	}
	if esbCMDB.searchCalled != 0 {
		t.Fatalf("expected direct SearchBusiness not to be called for ListAllBusiness, got %d", esbCMDB.searchCalled)
	}
}

func TestNewAllowsUseESBWithoutESBClient(t *testing.T) {
	if _, err := New(&cc.CMDBConfig{UseEsb: true}, nil); err != nil {
		t.Fatalf("new cmdb service should not require esb client until esb method is called: %v", err)
	}
}

func TestESBBusinessMethodsFailWhenESBClientMissing(t *testing.T) {
	svc, err := New(&cc.CMDBConfig{UseEsb: true}, nil)
	if err != nil {
		t.Fatalf("new cmdb service failed: %v", err)
	}

	if _, err = svc.SearchBusiness(context.Background(), &esbcmdb.SearchBizParams{}); err == nil ||
		!strings.Contains(err.Error(), "esb cmdb client is nil") {
		t.Fatalf("expected SearchBusiness to fail with missing esb cmdb client, got %v", err)
	}

	if _, err = svc.ListAllBusiness(context.Background()); err == nil ||
		!strings.Contains(err.Error(), "esb cmdb client is nil") {
		t.Fatalf("expected ListAllBusiness to fail with missing esb cmdb client, got %v", err)
	}
}
