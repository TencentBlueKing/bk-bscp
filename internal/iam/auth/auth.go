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

// Package auth NOTES
package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"

	"github.com/TencentBlueKing/bk-bscp/internal/components/bkcmdb"
	"github.com/TencentBlueKing/bk-bscp/internal/components/bkpaas"
	"github.com/TencentBlueKing/bk-bscp/internal/runtime/gwparser"
	"github.com/TencentBlueKing/bk-bscp/internal/serviced"
	"github.com/TencentBlueKing/bk-bscp/internal/space"
	esbcli "github.com/TencentBlueKing/bk-bscp/internal/thirdparty/esb/client"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/errf"
	"github.com/TencentBlueKing/bk-bscp/pkg/i18n"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/client"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/meta"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	"github.com/TencentBlueKing/bk-bscp/pkg/metrics"
	pbas "github.com/TencentBlueKing/bk-bscp/pkg/protocol/auth-server"
	"github.com/TencentBlueKing/bk-bscp/pkg/rest"
	"github.com/TencentBlueKing/bk-bscp/pkg/tools"
)

// Authorizer defines all the supported functionalities to do auth operation.
type Authorizer interface {
	// AuthorizeDecision if user has permission to the resources, returns auth status per resource and for all.
	AuthorizeDecision(kt *kit.Kit, resources ...*meta.ResourceAttribute) ([]*meta.Decision, bool, error)
	// Authorize authorize if user has permission to the resources.
	// If user is unauthorized, assign apply url and resources into error.
	Authorize(kt *kit.Kit, resources ...*meta.ResourceAttribute) error
	// UnifiedAuthentication API 鉴权中间件
	UnifiedAuthentication(next http.Handler) http.Handler
	// GrantResourceCreatorAction grant a user's resource creator action.
	GrantResourceCreatorAction(kt *kit.Kit, opts *client.GrantResourceCreatorActionOption) error
	// WebAuthentication 网页鉴权中间件
	WebAuthentication(webHost string) func(http.Handler) http.Handler
	// AppVerified App校验中间件, 需要放到 UnifiedAuthentication 后面, url 需要添加 {app_id} 变量
	AppVerified(next http.Handler) http.Handler
	// BizVerified 业务鉴权
	BizVerified(next http.Handler) http.Handler
	// ContentVerified 内容(上传下载)鉴权
	ContentVerified(next http.Handler) http.Handler
	// LogOut handler will build login url, client should make redirect
	LogOut(r *http.Request) *rest.UnauthorizedData
	// HasBiz 业务是否存在
	HasBiz(ctx context.Context, bizID uint32) bool
	// IAMVerify iam 验证
	IAMVerify(next http.Handler) http.Handler
}

// NewAuthorizer create an authorizer for iam authorize related operation.
func NewAuthorizer(sd serviced.Discover, tls cc.TLSConfig) (Authorizer, error) {
	opts := make([]grpc.DialOption, 0)

	// add dial load balancer.
	opts = append(opts, sd.LBRoundRobin())

	if !tls.Enable() {
		// dial without ssl
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		// dial with ssl.
		tlsC, err := tools.ClientTLSConfVerify(tls.InsecureSkipVerify, tls.CAFile, tls.CertFile, tls.KeyFile,
			tls.Password)
		if err != nil {
			return nil, fmt.Errorf("init client set tls config failed, err: %v", err)
		}

		cred := credentials.NewTLS(tlsC)
		opts = append(opts, grpc.WithTransportCredentials(cred))
	}

	// connect auth server.
	asConn, err := grpc.Dial(serviced.GrpcServiceDiscoveryName(cc.AuthServerName), opts...)
	if err != nil {
		logs.Errorf("dial auth server failed, err: %v", err)
		return nil, errf.New(errf.Unknown, fmt.Sprintf("dial auth server failed, err: %v", err))
	}

	authClient := pbas.NewAuthClient(asConn)
	resp, err := authClient.GetAuthConf(context.Background(), &pbas.GetAuthConfReq{})
	if err != nil {
		return nil, errors.Wrap(err, "get auth conf")
	}
	if validateErr := validateAuthConf(resp); validateErr != nil {
		klog.ErrorS(validateErr, "validate auth conf failed")
		return nil, fmt.Errorf("get auth conf invalid, err: %v", validateErr)
	}

	loginAuth := resp.GetLoginAuth()
	esb := resp.GetEsb()
	cmdbConf := resp.GetCmdb()

	conf := &cc.LoginAuthSettings{
		Host:      loginAuth.GetHost(),
		InnerHost: loginAuth.GetInnerHost(),
		Provider:  loginAuth.GetProvider(),
	}

	esbTLS := cc.TLSConfig{}
	if esb.GetTls() != nil {
		esbTLS = cc.TLSConfig{
			InsecureSkipVerify: esb.GetTls().GetInsecureSkipVerify(),
			CertFile:           esb.GetTls().GetCertFile(),
			KeyFile:            esb.GetTls().GetKeyFile(),
			CAFile:             esb.GetTls().GetCaFile(),
			Password:           esb.GetTls().GetPassword(),
		}
	}

	// init space manager
	esbSetting := &cc.Esb{
		Endpoints: esb.GetEndpoints(),
		AppCode:   esb.GetAppCode(),
		AppSecret: esb.GetAppSecret(),
		User:      esb.GetUser(),
		TLS:       esbTLS,
	}

	cmdbCfg := buildCMDBConfig(cmdbConf)

	authLoginClient := bkpaas.NewAuthLoginClient(conf)
	klog.InfoS("init authlogin client done", "host", conf.Host, "inner_host", conf.InnerHost, "provider", conf.Provider)
	esbCli, err := esbcli.NewClient(esbSetting, metrics.Register())
	if err != nil {
		return nil, fmt.Errorf("new esb client failed, err: %v", err)
	}
	cmdb, err := bkcmdb.New(cmdbCfg, esbCli)
	if err != nil {
		klog.ErrorS(err, "init cmdb client failed")
		return nil, fmt.Errorf("init cmdb client failed, err: %v", err)
	}
	spaceMgr, err := space.NewSpaceMgr(context.Background(), cmdb)
	if err != nil {
		return nil, fmt.Errorf("init space manager failed, err: %v", err)
	}

	authz := &authorizer{
		authClient:      authClient,
		authLoginClient: authLoginClient,
		gwParser:        nil,
		spaceMgr:        spaceMgr,
	}

	// 如果有公钥，初始化配置
	if resp.LoginAuth.GwPubkey != "" {
		gwParser, err := gwparser.NewJWTParser(resp.LoginAuth.GwPubkey)
		if err != nil {
			return nil, errors.Wrap(err, "init gw parser")
		}

		authz.gwParser = gwParser
		klog.InfoS("init gw parser done", "fingerprint", gwParser.Fingerprint())
	}

	return authz, nil
}

func buildCMDBConfig(cmdbConf *pbas.CMDB) *cc.CMDBConfig {
	return &cc.CMDBConfig{
		Host:       cmdbConf.GetHost(),
		AppCode:    cmdbConf.GetAppCode(),
		AppSecret:  cmdbConf.GetAppSecret(),
		BkUserName: cmdbConf.GetBkUserName(),
		UseEsb:     cc.G().CMDB.UseEsb,
	}
}

func validateAuthConf(resp *pbas.GetAuthConfResp) error {
	if resp == nil {
		return fmt.Errorf("get auth conf response is nil")
	}

	missingFields := make([]string, 0)

	loginAuth := resp.GetLoginAuth()
	if loginAuth == nil {
		missingFields = append(missingFields, "loginAuth")
	} else if strings.TrimSpace(loginAuth.GetHost()) == "" {
		missingFields = append(missingFields, "loginAuth.host")
	}

	esb := resp.GetEsb()
	if esb == nil {
		missingFields = append(missingFields, "esb")
	} else {
		if len(esb.GetEndpoints()) == 0 {
			missingFields = append(missingFields, "esb.endpoints")
		}
		if strings.TrimSpace(esb.GetAppCode()) == "" {
			missingFields = append(missingFields, "esb.appCode")
		}
		if strings.TrimSpace(esb.GetAppSecret()) == "" {
			missingFields = append(missingFields, "esb.appSecret")
		}
	}

	cmdb := resp.GetCmdb()
	if cmdb == nil {
		missingFields = append(missingFields, "cmdb")
	} else {
		if strings.TrimSpace(cmdb.GetHost()) == "" {
			missingFields = append(missingFields, "cmdb.host")
		}
		if strings.TrimSpace(cmdb.GetAppCode()) == "" {
			missingFields = append(missingFields, "cmdb.appCode")
		}
		if strings.TrimSpace(cmdb.GetAppSecret()) == "" {
			missingFields = append(missingFields, "cmdb.appSecret")
		}
	}

	if len(missingFields) > 0 {
		return fmt.Errorf("missing required auth config fields: %s", strings.Join(missingFields, ", "))
	}

	cmdbHost := strings.TrimSpace(cmdb.GetHost())
	parsedURL, err := url.Parse(cmdbHost)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return fmt.Errorf("invalid cmdb.host: %q, must be full url with scheme", cmdbHost)
	}

	return nil
}

type authorizer struct {
	// authClient auth server's client api
	authClient      pbas.AuthClient
	authLoginClient bkpaas.AuthLoginClient
	gwParser        gwparser.Parser
	spaceMgr        *space.Manager
}

// AuthorizeDecision if user has permission to the resources, returns auth status per resource and for all.
func (a authorizer) AuthorizeDecision(kt *kit.Kit, resources ...*meta.ResourceAttribute) (
	[]*meta.Decision, bool, error) {
	userInfo := &meta.UserInfo{UserName: kt.User}

	req := &pbas.AuthorizeBatchReq{
		User:      pbas.PbUserInfo(userInfo),
		Resources: pbas.PbResourceAttributes(resources),
	}

	resp, err := a.authClient.AuthorizeBatch(kt.RpcCtx(), req)
	if err != nil {
		logs.Errorf("authorize failed, req: %#v, err: %v, rid: %s", req, err, kt.Rid)
		return nil, false, err
	}

	authorized := true
	for _, decision := range resp.Decisions {
		if !decision.Authorized {
			authorized = false
			break
		}
	}

	Decisions := make([]*meta.Decision, len(req.Resources))
	for idx := range resp.Decisions {
		Decisions[idx] = &meta.Decision{
			Authorized: true,
		}
	}

	return pbas.Decisions(resp.Decisions), authorized, nil
}

// Authorize authorize if user has permission to the resources.
// If user is unauthorized, assign apply url and resources into error.
func (a authorizer) Authorize(kt *kit.Kit, resources ...*meta.ResourceAttribute) error {
	_, authorized, err := a.AuthorizeDecision(kt, resources...)
	if err != nil {
		return errf.New(errf.DoAuthorizeFailed, i18n.T(kt, "authorize failed"))
	}

	if authorized {
		return nil
	}

	req := &pbas.GetPermissionToApplyReq{
		Resources: pbas.PbResourceAttributes(resources),
	}

	permResp, err := a.authClient.GetPermissionToApply(kt.RpcCtx(), req)
	if err != nil {
		logs.Errorf("get permission to apply failed, req: %#v, err: %v, rid: %s", req, err, kt.Rid)
		return errf.New(errf.DoAuthorizeFailed, i18n.T(kt, "get permission to apply failed, err: %v", err))
	}

	st := status.New(codes.PermissionDenied, "permission denied")
	details := pbas.ApplyDetail{
		Resources: []*pbas.BasicDetail{},
		ApplyUrl:  permResp.ApplyUrl,
	}
	for _, action := range permResp.Permission.Actions {
		for _, resourceType := range action.RelatedResourceTypes {
			for _, instance := range resourceType.Instances {
				for _, i := range instance.Instances {
					if i.Type != resourceType.Type {
						continue
					}
					details.Resources = append(details.Resources, &pbas.BasicDetail{
						Type:         resourceType.Type,
						TypeName:     resourceType.TypeName,
						Action:       action.Id,
						ActionName:   action.Name,
						ResourceId:   i.Id,
						ResourceName: i.Name,
					})
				}
			}
		}
	}
	st, err = st.WithDetails(&details)
	if err != nil {
		logs.Errorf("with details failed, err: %v", err)
		return errf.New(errf.PermissionDenied, i18n.T(kt, "grpc status with details failed, err: %v", err))
	}
	return st.Err()
}

// GrantResourceCreatorAction grant a user's resource creator action.
func (a authorizer) GrantResourceCreatorAction(kt *kit.Kit, opts *client.GrantResourceCreatorActionOption) error {
	req := pbas.PbGrantResourceCreatorActionOption(opts)
	_, err := a.authClient.GrantResourceCreatorAction(kt.RpcCtx(), req)
	return err
}

// LogOut handler will build login url, client should make redirect
func (a authorizer) LogOut(r *http.Request) *rest.UnauthorizedData {
	loginURL, loginPlainURL := a.authLoginClient.BuildLoginURL(r)
	return &rest.UnauthorizedData{LoginURL: loginURL, LoginPlainURL: loginPlainURL}
}

// HasBiz 业务是否存在
func (a authorizer) HasBiz(ctx context.Context, bizID uint32) bool {
	return a.spaceMgr.HasCMDBSpace(ctx, strconv.FormatUint(uint64(bizID), 10))
}
