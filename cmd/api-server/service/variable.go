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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"gopkg.in/yaml.v3"

	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	pbcs "github.com/TencentBlueKing/bk-bscp/pkg/protocol/config-server"
	pbapp "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/app"
	pbrelease "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/release"
	pbtv "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/template-variable"
	"github.com/TencentBlueKing/bk-bscp/pkg/rest"
)

type variableService struct {
	cfgClient pbcs.ConfigClient
}

func newVariableService(cfgClient pbcs.ConfigClient) *variableService {
	s := &variableService{
		cfgClient: cfgClient,
	}
	return s
}

// ExportGlobalVariables exports global variables.
func (s *variableService) ExportGlobalVariables(w http.ResponseWriter, r *http.Request) {
	kt := kit.MustGetKit(r.Context())

	format := r.URL.Query().Get("format")

	vars, err := s.cfgClient.ListTemplateVariables(kt.RpcCtx(), &pbcs.ListTemplateVariablesReq{
		BizId: kt.BizID,
		All:   true,
	})
	if err != nil {
		logs.Errorf("list template variables failed, err: %s", err)
		_ = render.Render(w, r, rest.BadRequest(err))
		return
	}
	var vs []*pbtv.TemplateVariableSpec
	for _, v := range vars.Details {
		vs = append(vs, v.Spec)
	}

	var exporter VariableExporter

	outData := variablesToOutData(vs)
	switch format {
	case "yaml":
		exporter = &YAMLVariableExporter{OutData: outData}
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%d_global_variable.yaml", kt.BizID))
		w.Header().Set("Content-Type", "application/x-yaml")
	case "json":
		exporter = &JSONVariableExporter{OutData: outData}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%d_global_variable.json", kt.BizID))
	default:
		_ = render.Render(w, r, rest.BadRequest(errors.New("invalid format")))
		return
	}

	content, err := exporter.VariableExport()
	if err != nil {
		logs.Errorf("export variable fail, err: %v", err)
		_ = render.Render(w, r, rest.BadRequest(err))
	}
	_, err = w.Write(content)
	if err != nil {
		logs.Errorf("Error writing response:%s", err)
		_ = render.Render(w, r, rest.BadRequest(err))
	}
}

// ExportAppVariables exports app variables.
func (s *variableService) ExportAppVariables(w http.ResponseWriter, r *http.Request) {
	kt := kit.MustGetKit(r.Context())

	format := r.URL.Query().Get("format")

	vars, err := s.cfgClient.ListAppTmplVariables(kt.RpcCtx(), &pbcs.ListAppTmplVariablesReq{
		BizId: kt.BizID,
		AppId: kt.AppID,
	})
	if err != nil {
		logs.Errorf("list app template variables failed, err: %s", err)
		_ = render.Render(w, r, rest.BadRequest(err))
		return
	}

	var app *pbapp.App
	if app, err = s.cfgClient.GetApp(kt.RpcCtx(), &pbcs.GetAppReq{
		BizId: kt.BizID,
		AppId: kt.AppID,
	}); err != nil {
		logs.Errorf("get app failed, err: %s", err)
		_ = render.Render(w, r, rest.BadRequest(err))
		return
	}

	var exporter VariableExporter

	outData := variablesToOutData(vars.Details)
	switch format {
	case "yaml":
		exporter = &YAMLVariableExporter{OutData: outData}
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%d_%s_variable.yaml",
			kt.BizID, app.Spec.Name))
		w.Header().Set("Content-Type", "application/x-yaml")
	case "json":
		exporter = &JSONVariableExporter{OutData: outData}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%d_%s_variable.json",
			kt.BizID, app.Spec.Name))
	default:
		_ = render.Render(w, r, rest.BadRequest(errors.New("invalid format")))
		return
	}

	content, err := exporter.VariableExport()
	if err != nil {
		logs.Errorf("export variable fail, err: %v", err)
		_ = render.Render(w, r, rest.BadRequest(err))
	}
	_, err = w.Write(content)
	if err != nil {
		logs.Errorf("Error writing response:%s", err)
		_ = render.Render(w, r, rest.BadRequest(err))
	}
}

// ExportReleasedAppVariables exports released app variables.
func (s *variableService) ExportReleasedAppVariables(w http.ResponseWriter, r *http.Request) {
	kt := kit.MustGetKit(r.Context())
	format := r.URL.Query().Get("format")
	releaseIDStr := chi.URLParam(r, "release_id")
	releaseID, _ := strconv.Atoi(releaseIDStr)

	vars, err := s.cfgClient.ListReleasedAppTmplVariables(kt.RpcCtx(), &pbcs.ListReleasedAppTmplVariablesReq{
		BizId:     kt.BizID,
		AppId:     kt.AppID,
		ReleaseId: uint32(releaseID),
	})
	if err != nil {
		_ = render.Render(w, r, rest.BadRequest(err))
		return
	}

	var app *pbapp.App
	if app, err = s.cfgClient.GetApp(kt.RpcCtx(), &pbcs.GetAppReq{
		BizId: kt.BizID,
		AppId: kt.AppID,
	}); err != nil {
		logs.Errorf("get app failed, err: %s", err)
		_ = render.Render(w, r, rest.BadRequest(err))
		return
	}

	var rel *pbrelease.Release
	if rel, err = s.cfgClient.GetRelease(kt.RpcCtx(), &pbcs.GetReleaseReq{
		BizId:     kt.BizID,
		AppId:     kt.AppID,
		ReleaseId: uint32(releaseID),
	}); err != nil {
		logs.Errorf("get release failed, err: %s", err)
		_ = render.Render(w, r, rest.BadRequest(err))
		return
	}

	var exporter VariableExporter

	outData := variablesToOutData(vars.Details)
	switch format {
	case "yaml":
		exporter = &YAMLVariableExporter{OutData: outData}
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%d_%s_%s_variable.yaml",
			kt.BizID, app.Spec.Name, rel.Spec.Name))
		w.Header().Set("Content-Type", "application/x-yaml")
	case "json":
		exporter = &JSONVariableExporter{OutData: outData}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%d_%s_%s_variable.json",
			kt.BizID, app.Spec.Name, rel.Spec.Name))
	default:
		_ = render.Render(w, r, rest.BadRequest(errors.New("invalid format")))
		return
	}

	content, err := exporter.VariableExport()
	if err != nil {
		logs.Errorf("export variable fail, err: %v", err)
		_ = render.Render(w, r, rest.BadRequest(err))
	}
	_, err = w.Write(content)
	if err != nil {
		logs.Errorf("Error writing response:%s", err)
		_ = render.Render(w, r, rest.BadRequest(err))
	}
}

// VariableExporter The Exporter interface defines methods for exporting files.
type VariableExporter interface {
	VariableExport() ([]byte, error)
}

// YAMLVariableExporter implements the Exporter interface for exporting YAML files.
type YAMLVariableExporter struct {
	OutData map[string]interface{}
}

// VariableExport method implements the Exporter interface, exporting data as a byte slice in YAML format.
func (ym *YAMLVariableExporter) VariableExport() ([]byte, error) {
	buffer := &bytes.Buffer{}
	encoder := yaml.NewEncoder(buffer)
	// 设置缩进
	encoder.SetIndent(2)
	defer func() {
		_ = encoder.Close()
	}()
	err := encoder.Encode(ym.OutData)
	return buffer.Bytes(), err
}

// JSONVariableExporter implements the Exporter interface for exporting JSON files.
type JSONVariableExporter struct {
	OutData map[string]interface{}
}

// VariableExport method implements the Exporter interface, exporting data as a byte slice in JSON format.
func (js *JSONVariableExporter) VariableExport() ([]byte, error) {
	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false)
	// Set indent for pretty-printed JSON
	// adds two spaces for indentation
	encoder.SetIndent("", "  ")
	err := encoder.Encode(js.OutData)
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func variablesToOutData(vars []*pbtv.TemplateVariableSpec) map[string]interface{} {
	d := map[string]interface{}{}
	for _, v := range vars {
		var value interface{}
		value = v.DefaultVal
		if v.Type == string(table.NumberVar) {
			i, _ := strconv.Atoi(v.DefaultVal)
			value = i
		}

		d[v.Name] = map[string]interface{}{
			"variable_type": v.Type,
			"value":         value,
			"memo":          v.Memo,
		}
	}

	return d
}
