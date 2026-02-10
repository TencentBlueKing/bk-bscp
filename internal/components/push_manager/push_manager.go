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

// Package pushmanager provides bcs push manager api client.
package pushmanager

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"

	"github.com/go-resty/resty/v2"

	"github.com/TencentBlueKing/bk-bscp/internal/components"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

const apiPrefix = "/pushmanager/api/v1"

// resourcePath represents a collection resource path
// HTTP Method determines the action (CRUD)
type resourcePath string

const (
	resourcePushEvent     resourcePath = "/push_events"
	resourcePushTemplate  resourcePath = "/push_templates"
	resourcePushWhitelist resourcePath = "/push_whitelists"
)

func buildCollectionURL(host, domain string, res resourcePath) string {
	return fmt.Sprintf("%s%s/domains/%s%s", host, apiPrefix, domain, res)
}

func buildResourceURL(host, domain string, res resourcePath, id string) string {
	return fmt.Sprintf("%s%s/domains/%s%s/%s", host, apiPrefix, domain, res, id)
}

func appendQuery(rawURL string, values url.Values) string {
	if len(values) == 0 {
		return rawURL
	}
	return rawURL + "?" + values.Encode()
}

func encodePagination(p Pagination) url.Values {
	if p.Page < 1 {
		p.Page = DefaultPage
	}
	if p.PageSize <= 0 {
		p.PageSize = DefaultPageSize
	}
	v := url.Values{}
	v.Set("page", strconv.Itoa(p.Page))
	v.Set("page_size", strconv.Itoa(p.PageSize))

	return v
}

// pushManagerService push manager client
type pushManagerService struct {
	cc.PushProviderConfig
}

func (p *pushManagerService) doRequest(ctx context.Context, method components.HTTPMethod,
	url string, body any, result any) error {

	req := components.GetClient().SetDebug(false).R().
		SetContext(ctx).
		SetAuthToken(p.Token).
		SetBody(body)

	var resp *resty.Response
	var err error

	switch method {
	case components.GET:
		resp, err = req.Get(url)
	case components.POST:
		resp, err = req.Post(url)
	case components.PUT:
		resp, err = req.Put(url)
	case components.DELETE:
		resp, err = req.Delete(url)
	default:
		return fmt.Errorf("http method %s not supported", method)
	}

	if err != nil {
		return err
	}

	if result == nil {
		return nil
	}

	if err := json.Unmarshal(resp.Body(), result); err != nil {
		logs.Errorf("json unmarshal failed, err=%v, resp=%s", err, string(resp.Body()))
		return err
	}

	return nil
}

// CreatePushEvent implements [Service].
func (p *pushManagerService) CreatePushEvent(ctx context.Context, req *CreatePushEventRequest) (*CreatePushEventResponse, error) {
	if req == nil {
		return nil, errors.New("CreatePushEvent: request is nil")
	}
	if err := req.validate(); err != nil {
		logs.Errorf("[CreatePushEvent] validate failed: %v", err)
		return nil, err
	}

	url := buildCollectionURL(p.Host, p.Domain, resourcePushEvent)
	resp := new(CreatePushEventResponse)

	if err := p.doRequest(ctx, components.POST, url, req, resp); err != nil {
		logs.Errorf("[CreatePushEvent] request failed: domain=%s event_id=%s err=%v",
			p.Domain, req.Event.EventID, err)
		return nil, err
	}

	logs.Infof("[CreatePushEvent] success: domain=%s event_id=%s", p.Domain, resp.EventID)
	return resp, nil
}

// CreatePushTemplate implements [Service].
func (p *pushManagerService) CreatePushTemplate(ctx context.Context, req *CreatePushTemplateRequest) (*BaseResponse, error) {
	if req == nil {
		return nil, errors.New("CreatePushTemplate: request is nil")
	}
	if err := req.validate(); err != nil {
		return nil, err
	}

	url := buildCollectionURL(p.Host, p.Domain, resourcePushTemplate)
	resp := new(BaseResponse)

	if err := p.doRequest(ctx, components.POST, url, req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// CreatePushWhitelist implements [Service].
func (p *pushManagerService) CreatePushWhitelist(ctx context.Context, req *CreatePushWhitelistRequest) (*BaseResponse, error) {
	if req == nil {
		return nil, errors.New("CreatePushWhitelist: request is nil")
	}
	if err := req.validate(); err != nil {
		return nil, err
	}

	url := buildCollectionURL(p.Host, p.Domain, resourcePushWhitelist)
	resp := new(BaseResponse)

	if err := p.doRequest(ctx, components.POST, url, req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// DeletePushEvent implements [Service].
func (p *pushManagerService) DeletePushEvent(ctx context.Context, domain, eventID string) (*BaseResponse, error) {
	if err := validateID(domain, eventID, "event_id"); err != nil {
		return nil, fmt.Errorf("DeletePushEvent: %w", err)
	}
	url := buildResourceURL(p.Host, domain, resourcePushEvent, eventID)
	resp := new(BaseResponse)
	return resp, p.doRequest(ctx, components.DELETE, url, nil, resp)
}

// DeletePushTemplate implements [Service].
func (p *pushManagerService) DeletePushTemplate(ctx context.Context, domain, templateID string) (*BaseResponse, error) {
	if err := validateID(domain, templateID, "template_id"); err != nil {
		return nil, fmt.Errorf("DeletePushTemplate: %w", err)
	}
	url := buildResourceURL(p.Host, domain, resourcePushTemplate, templateID)
	resp := new(BaseResponse)
	return resp, p.doRequest(ctx, components.DELETE, url, nil, resp)
}

// DeletePushTemplate implements [Service].
func (p *pushManagerService) DeletePushWhitelist(ctx context.Context, domain, whitelistID string) (*BaseResponse, error) {
	if err := validateID(domain, whitelistID, "whitelist_id"); err != nil {
		return nil, fmt.Errorf("DeletePushWhitelist: %w", err)
	}
	url := buildResourceURL(p.Host, domain, resourcePushWhitelist, whitelistID)
	resp := new(BaseResponse)
	return resp, p.doRequest(ctx, components.DELETE, url, nil, resp)
}

// GetPushEvent implements [Service].
func (p *pushManagerService) GetPushEvent(ctx context.Context, domain, eventID string) (*GetPushEventResponse, error) {
	if err := validateID(domain, eventID, "event_id"); err != nil {
		return nil, fmt.Errorf("GetPushEvent: %w", err)
	}
	url := buildResourceURL(p.Host, domain, resourcePushEvent, eventID)
	resp := new(GetPushEventResponse)
	return resp, p.doRequest(ctx, components.GET, url, nil, resp)
}

// GetPushTemplate implements [Service].
func (p *pushManagerService) GetPushTemplate(ctx context.Context, domain, templateID string) (*GetPushTemplateResponse, error) {
	if err := validateID(domain, templateID, "template_id"); err != nil {
		return nil, fmt.Errorf("GetPushTemplate: %w", err)
	}
	url := buildResourceURL(p.Host, domain, resourcePushTemplate, templateID)
	resp := new(GetPushTemplateResponse)
	return resp, p.doRequest(ctx, components.GET, url, nil, resp)
}

// GetPushWhitelist implements [Service].
func (p *pushManagerService) GetPushWhitelist(ctx context.Context, domain, whitelistID string) (*GetPushWhitelistResponse, error) {
	if err := validateID(domain, whitelistID, "whitelist_id"); err != nil {
		return nil, fmt.Errorf("GetPushWhitelist: %w", err)
	}
	url := buildResourceURL(p.Host, domain, resourcePushWhitelist, whitelistID)
	resp := new(GetPushWhitelistResponse)
	return resp, p.doRequest(ctx, components.GET, url, nil, resp)
}

// ListPushEvents implements [Service].
func (p *pushManagerService) ListPushEvents(ctx context.Context, domain string, query *ListPushEventsRequest) (*ListPushEventsResponse, error) {
	if domain == "" {
		return nil, errors.New("ListPushEvents: domain is required")
	}
	values := encodePagination(query.Pagination)
	url := appendQuery(buildCollectionURL(p.Host, domain, resourcePushEvent), values)
	resp := new(ListPushEventsResponse)
	return resp, p.doRequest(ctx, components.GET, url, nil, resp)
}

// ListPushTemplates implements [Service].
func (p *pushManagerService) ListPushTemplates(ctx context.Context, domain string, query *ListPushTemplatesRequest) (*ListPushTemplatesResponse, error) {
	if domain == "" {
		return nil, errors.New("ListPushTemplates: domain is required")
	}
	values := encodePagination(query.Pagination)
	url := appendQuery(buildCollectionURL(p.Host, domain, resourcePushTemplate), values)
	resp := new(ListPushTemplatesResponse)
	return resp, p.doRequest(ctx, components.GET, url, nil, resp)
}

// ListPushWhitelists implements [Service].
func (p *pushManagerService) ListPushWhitelists(ctx context.Context, domain string, query *ListPushWhitelistsRequest) (*ListPushWhitelistsResponse, error) {
	if domain == "" {
		return nil, errors.New("ListPushWhitelists: domain is required")
	}
	values := encodePagination(query.Pagination)
	url := appendQuery(buildCollectionURL(p.Host, domain, resourcePushWhitelist), values)
	resp := new(ListPushWhitelistsResponse)
	return resp, p.doRequest(ctx, components.GET, url, nil, resp)
}

// UpdatePushEvent implements [Service].
func (p *pushManagerService) UpdatePushEvent(ctx context.Context, domain, eventID string, req *UpdatePushEventRequest) (*BaseResponse, error) {
	if err := validateID(domain, eventID, "event_id"); err != nil {
		return nil, fmt.Errorf("UpdatePushEvent: %w", err)
	}
	if req == nil {
		return nil, errors.New("UpdatePushEvent: request is nil")
	}
	url := buildResourceURL(p.Host, domain, resourcePushEvent, eventID)
	resp := new(BaseResponse)
	return resp, p.doRequest(ctx, components.PUT, url, req, resp)
}

// UpdatePushTemplate implements [Service].
func (p *pushManagerService) UpdatePushTemplate(ctx context.Context, domain, templateID string, req *UpdatePushTemplateRequest) (*BaseResponse, error) {
	if err := validateID(domain, templateID, "template_id"); err != nil {
		return nil, fmt.Errorf("UpdatePushTemplate: %w", err)
	}
	if req == nil {
		return nil, errors.New("UpdatePushTemplate: request is nil")
	}
	url := buildResourceURL(p.Host, domain, resourcePushTemplate, templateID)
	resp := new(BaseResponse)
	return resp, p.doRequest(ctx, components.PUT, url, req, resp)
}

// UpdatePushWhitelist implements [Service].
func (p *pushManagerService) UpdatePushWhitelist(ctx context.Context, domain, whitelistID string, req *UpdatePushWhitelistRequest) (*BaseResponse, error) {
	if err := validateID(domain, whitelistID, "whitelist_id"); err != nil {
		return nil, fmt.Errorf("UpdatePushWhitelist: %w", err)
	}
	if req == nil {
		return nil, errors.New("UpdatePushWhitelist: request is nil")
	}
	url := buildResourceURL(p.Host, domain, resourcePushWhitelist, whitelistID)
	resp := new(BaseResponse)
	return resp, p.doRequest(ctx, components.PUT, url, req, resp)
}
