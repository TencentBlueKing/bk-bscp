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

package service

import (
	"context"

	"github.com/TencentBlueKing/bk-bscp/pkg/iam/meta"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	pbcs "github.com/TencentBlueKing/bk-bscp/pkg/protocol/config-server"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
)

// ListConfigTemplate implements pbcs.ConfigServer.
func (s *Service) ListConfigTemplate(ctx context.Context, req *pbcs.ListConfigTemplateReq) (
	*pbcs.ListConfigTemplateResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}
	resp, err := s.client.DS.ListConfigTemplate(grpcKit.RpcCtx(), &pbds.ListConfigTemplateReq{
		BizId:  req.GetBizId(),
		Search: req.GetSearch(),
		All:    req.GetAll(),
		Limit:  req.GetLimit(),
		Start:  req.GetStart(),
	})
	if err != nil {
		return nil, err
	}

	return &pbcs.ListConfigTemplateResp{
		Count:   resp.GetCount(),
		Details: resp.GetDetails(),
		TemplateSpace: &pbcs.ListConfigTemplateResp_Item{
			Id:   resp.GetTemplateSpace().GetId(),
			Name: resp.GetTemplateSpace().GetName(),
		},
		TemplateSet: &pbcs.ListConfigTemplateResp_Item{
			Id:   resp.GetTemplateSet().GetId(),
			Name: resp.GetTemplateSet().GetName(),
		},
	}, nil
}

// BizTopo implements pbcs.ConfigServer.
func (s *Service) BizTopo(ctx context.Context, req *pbcs.BizTopoReq) (*pbcs.BizTopoResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}

	resp, err := s.client.DS.BizTopo(grpcKit.RpcCtx(), &pbds.BizTopoReq{
		BizId: req.GetBizId(),
	})
	if err != nil {
		return nil, err
	}

	return &pbcs.BizTopoResp{
		BizTopoNodes: resp.GetBizTopoNodes(),
	}, nil
}

// ServiceTemplate implements pbcs.ConfigServer.
func (s *Service) ServiceTemplate(ctx context.Context, req *pbcs.ServiceTemplateReq) (*pbcs.ServiceTemplateResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}
	resp, err := s.client.DS.ServiceTemplate(grpcKit.RpcCtx(), &pbds.ServiceTemplateReq{
		BizId: req.GetBizId(),
	})
	if err != nil {
		return nil, err
	}

	return &pbcs.ServiceTemplateResp{
		ServiceTemplates: resp.GetServiceTemplates(),
	}, nil
}

// ProcessTemplate implements pbcs.ConfigServer.
func (s *Service) ProcessTemplate(ctx context.Context, req *pbcs.ProcessTemplateReq) (*pbcs.ProcessTemplateResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}
	resp, err := s.client.DS.ProcessTemplate(grpcKit.RpcCtx(), &pbds.ProcessTemplateReq{
		BizId:             req.GetBizId(),
		ServiceTemplateId: req.GetServiceTemplateId(),
	})
	if err != nil {
		return nil, err
	}

	return &pbcs.ProcessTemplateResp{
		ProcessTemplates: resp.GetProcessTemplates(),
	}, nil
}

// ProcessInstance implements pbcs.ConfigServer.
func (s *Service) ProcessInstance(ctx context.Context, req *pbcs.ProcessInstanceReq) (*pbcs.ProcessInstanceResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}
	resp, err := s.client.DS.ProcessInstance(grpcKit.RpcCtx(), &pbds.ProcessInstanceReq{
		BizId:             req.GetBizId(),
		ServiceInstanceId: req.GetServiceInstanceId(),
	})
	if err != nil {
		return nil, err
	}

	return &pbcs.ProcessInstanceResp{
		ProcessInstances: resp.GetProcessInstances(),
	}, nil
}

// ServiceInstance implements pbcs.ConfigServer.
func (s *Service) ServiceInstance(ctx context.Context, req *pbcs.ServiceInstanceReq) (*pbcs.ServiceInstanceResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}
	resp, err := s.client.DS.ServiceInstance(grpcKit.RpcCtx(), &pbds.ServiceInstanceReq{
		BizId:    req.GetBizId(),
		ModuleId: req.GetModuleId(),
	})
	if err != nil {
		return nil, err
	}

	return &pbcs.ServiceInstanceResp{
		ServiceInstances: resp.GetServiceInstances(),
	}, nil
}

// CreateConfigTemplate implements pbcs.ConfigServer.
func (s *Service) CreateConfigTemplate(ctx context.Context, req *pbcs.CreateConfigTemplateReq) (*pbcs.CreateConfigTemplateResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}
	resp, err := s.client.DS.CreateConfigTemplate(grpcKit.RpcCtx(), &pbds.CreateConfigTemplateReq{
		BizId:           req.GetBizId(),
		Name:            req.GetName(),
		FileName:        req.GetFileName(),
		FilePath:        req.GetFilePath(),
		Memo:            req.GetMemo(),
		User:            req.GetUser(),
		UserGroup:       req.GetUserGroup(),
		Privilege:       req.GetPrivilege(),
		Sign:            req.GetSign(),
		ByteSize:        req.GetByteSize(),
		Md5:             req.GetMd5(),
		Charset:         req.GetCharset(),
		HighlightStyle:  req.GetHighlightStyle(),
		FileMode:        req.GetFileMode(),
		RevisionName:    req.GetRevisionName(),
		TemplateSpaceId: req.GetTemplateSpaceId(),
	})
	if err != nil {
		return nil, err
	}

	return &pbcs.CreateConfigTemplateResp{
		Id: resp.GetId(),
	}, nil
}

// ConfigTemplateVariable implements pbcs.ConfigServer.
func (s *Service) ConfigTemplateVariable(ctx context.Context, req *pbcs.ConfigTemplateVariableReq) (*pbcs.ConfigTemplateVariableResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}
	resp, err := s.client.DS.ConfigTemplateVariable(grpcKit.RpcCtx(), &pbds.ConfigTemplateVariableReq{
		Id: req.GetBizId(),
	})
	if err != nil {
		return nil, err
	}

	return &pbcs.ConfigTemplateVariableResp{
		ConfigTemplateVariables: resp.GetConfigTemplateVariables(),
	}, nil
}

// BindProcessInstance implements pbcs.ConfigServer.
func (s *Service) BindProcessInstance(ctx context.Context, req *pbcs.BindProcessInstanceReq) (
	*pbcs.BindProcessInstanceResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}
	resp, err := s.client.DS.BindProcessInstance(grpcKit.RpcCtx(), &pbds.BindProcessInstanceReq{
		BizId:                req.GetBizId(),
		ConfigTemplateId:     req.GetConfigTemplateId(),
		CcTemplateProcessIds: req.GetCcTemplateProcessIds(),
		CcProcessIds:         req.GetCcProcessIds(),
	})
	if err != nil {
		return nil, err
	}

	return &pbcs.BindProcessInstanceResp{
		Id: resp.GetId(),
	}, nil
}

// PreviewBindProcessInstance implements pbcs.ConfigServer.
func (s *Service) PreviewBindProcessInstance(ctx context.Context, req *pbcs.PreviewBindProcessInstanceReq) (*pbcs.PreviewBindProcessInstanceResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}
	resp, err := s.client.DS.PreviewBindProcessInstance(grpcKit.RpcCtx(), &pbds.PreviewBindProcessInstanceReq{
		BizId:            req.GetBizId(),
		ConfigTemplateId: req.GetConfigTemplateId(),
	})
	if err != nil {
		return nil, err
	}

	return &pbcs.PreviewBindProcessInstanceResp{
		TemplateProcesses: resp.GetTemplateProcesses(),
		InstanceProcesses: resp.GetInstanceProcesses(),
	}, nil
}

// UpdateConfigTemplate implements pbcs.ConfigServer.
func (s *Service) UpdateConfigTemplate(ctx context.Context, req *pbcs.UpdateConfigTemplateReq) (*pbcs.UpdateConfigTemplateResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}
	_, err := s.client.DS.UpdateConfigTemplate(grpcKit.RpcCtx(), &pbds.UpdateConfigTemplateReq{
		BizId:            req.GetBizId(),
		ConfigTemplateId: req.GetConfigTemplateId(),
		Name:             req.GetName(),
		Memo:             req.GetMemo(),
		User:             req.GetUser(),
		UserGroup:        req.GetUserGroup(),
		Privilege:        req.GetPrivilege(),
		Sign:             req.GetSign(),
		ByteSize:         req.GetByteSize(),
		Md5:              req.GetMd5(),
		Charset:          req.GetCharset(),
		HighlightStyle:   req.GetHighlightStyle(),
		FileMode:         req.GetFileMode(),
		RevisionName:     req.GetRevisionName(),
	})

	if err != nil {
		return nil, err
	}

	return &pbcs.UpdateConfigTemplateResp{}, nil
}

// GetConfigTemplate implements pbcs.ConfigServer.
func (s *Service) GetConfigTemplate(ctx context.Context, req *pbcs.GetConfigTemplateReq) (*pbcs.GetConfigTemplateResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}

	resp, err := s.client.DS.GetConfigTemplate(grpcKit.RpcCtx(), &pbds.GetConfigTemplateReq{
		BizId:            req.GetBizId(),
		ConfigTemplateId: req.GetConfigTemplateId(),
	})
	if err != nil {
		return nil, err
	}

	return &pbcs.GetConfigTemplateResp{
		BindTemplate: resp.GetBindTemplate(),
	}, nil
}

// DeleteConfigTemplate implements pbcs.ConfigServer.
func (s *Service) DeleteConfigTemplate(ctx context.Context, req *pbcs.DeleteConfigTemplateReq) (*pbcs.DeleteConfigTemplateResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}

	_, err := s.client.DS.DeleteConfigTemplate(grpcKit.RpcCtx(), &pbds.DeleteConfigTemplateReq{
		BizId:            req.GetBizId(),
		ConfigTemplateId: req.GetConfigTemplateId(),
	})
	if err != nil {
		return nil, err
	}

	return &pbcs.DeleteConfigTemplateResp{}, nil
}
