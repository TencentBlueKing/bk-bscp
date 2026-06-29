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

package config

import "testing"

func baseValidConfig() *Config {
	return &Config{
		Source: SourceConfig{MySQL: MySQLConfig{
			Endpoints: []string{"127.0.0.1:33060"}, Database: "src", User: "u"}},
		Target: TargetConfig{MySQL: MySQLConfig{
			Endpoints: []string{"127.0.0.1:3306"}, Database: "dst", User: "u"}},
		GSE: GSEConfig{Endpoint: "http://gse", AppCode: "code", AppSecret: "secret"},
	}
}

func TestValidateRequiresGSEConfig(t *testing.T) {
	cases := []struct {
		name string
		gse  GSEConfig
	}{
		{"missing endpoint", GSEConfig{AppCode: "code", AppSecret: "secret"}},
		{"missing app_code", GSEConfig{Endpoint: "http://gse", AppSecret: "secret"}},
		{"missing app_secret", GSEConfig{Endpoint: "http://gse", AppCode: "code"}},
		{"all empty", GSEConfig{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg := baseValidConfig()
			cfg.GSE = c.gse
			if err := cfg.Validate(); err == nil {
				t.Fatalf("expected error when gse config is incomplete (%s)", c.name)
			}
		})
	}
}

func TestValidatePassesWithGSEConfig(t *testing.T) {
	cfg := baseValidConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
