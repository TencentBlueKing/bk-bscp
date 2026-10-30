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

// Package service NOTES
package service

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	bkiam "github.com/TencentBlueKing/iam-go-sdk"
	bkiamlogger "github.com/TencentBlueKing/iam-go-sdk/logger"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	structpb "google.golang.org/protobuf/types/known/structpb"

	"github.com/TencentBlueKing/bk-bscp/cmd/auth-server/options"
	"github.com/TencentBlueKing/bk-bscp/cmd/auth-server/service/auth"
	"github.com/TencentBlueKing/bk-bscp/cmd/auth-server/service/iam"
	"github.com/TencentBlueKing/bk-bscp/cmd/auth-server/service/iamv4"
	"github.com/TencentBlueKing/bk-bscp/cmd/auth-server/service/initial"
	confsvc "github.com/TencentBlueKing/bk-bscp/cmd/config-server/service"
	"github.com/TencentBlueKing/bk-bscp/internal/components/bkcmdb"
	"github.com/TencentBlueKing/bk-bscp/internal/components/bkpaas"
	"github.com/TencentBlueKing/bk-bscp/internal/iam/apigw"
	iamauth "github.com/TencentBlueKing/bk-bscp/internal/iam/auth"
	"github.com/TencentBlueKing/bk-bscp/internal/rest/view/webannotation"
	"github.com/TencentBlueKing/bk-bscp/internal/serviced"
	"github.com/TencentBlueKing/bk-bscp/internal/space"
	esbcli "github.com/TencentBlueKing/bk-bscp/internal/thirdparty/esb/client"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/errf"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/client"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/meta"
	pkgauth "github.com/TencentBlueKing/bk-bscp/pkg/iam/sdk/auth"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/sys"
	authv4 "github.com/TencentBlueKing/bk-bscp/pkg/iam/v4/auth"
	clientv4 "github.com/TencentBlueKing/bk-bscp/pkg/iam/v4/client"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	"github.com/TencentBlueKing/bk-bscp/pkg/metrics"
	pbas "github.com/TencentBlueKing/bk-bscp/pkg/protocol/auth-server"
	pbcs "github.com/TencentBlueKing/bk-bscp/pkg/protocol/config-server"
	base "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/base"
	basepb "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/base"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
	"github.com/TencentBlueKing/bk-bscp/pkg/tools"
)

// Service do all the data service's work
type Service struct {
	client  *ClientSet
	gateway *gateway
	// disableAuth defines whether iam authorization is disabled
	disableAuth bool
	// disableWriteOpt defines which biz's write operation needs to be disabled
	disableWriteOpt *options.DisableWriteOption
	iamSettings     cc.IAM
	// iam logic module.
	iam *iam.IAM
	// iamV4Lgc iam v4 资源回调的处理逻辑，仅当 iam.version 为 v4 时非空
	iamV4Lgc *iamv4.IAM
	// initial logic module.
	initial *initial.Initial
	// auth logic module.
	auth     *auth.Auth
	spaceMgr *space.Manager
	pubKey   string
}

// NewService create a service instance.
func NewService(sd serviced.Discover, iamSettings cc.IAM, disableAuth bool,
	disableWriteOpt *options.DisableWriteOption) (*Service, error) {

	client, err := newClientSet(sd, cc.AuthServer().Network.TLS, iamSettings, disableAuth)
	if err != nil {
		return nil, fmt.Errorf("new client set failed, err: %v", err)
	}

	state, ok := sd.(serviced.State)
	if !ok {
		return nil, errors.New("discover convert state failed")
	}
	gateway, err := newGateway(state, client.sys)
	if err != nil {
		return nil, fmt.Errorf("new gateway failed, err: %v", err)
	}

	spaceMgr, err := space.NewSpaceMgr(context.Background(), client.cmdb)
	if err != nil {
		return nil, errors.Wrap(err, "init space mgr")
	}

	s := &Service{
		client:          client,
		gateway:         gateway,
		disableAuth:     disableAuth,
		disableWriteOpt: disableWriteOpt,
		iamSettings:     iamSettings,
		spaceMgr:        spaceMgr,
	}

	if errH := s.handlerAutoRegister(); errH != nil {
		return nil, errH
	}

	if err = s.initLogicModule(); err != nil {
		return nil, err
	}

	return s, nil
}

// 注册网关
func (s *Service) handlerAutoRegister() error {
	s.pubKey = cc.AuthServer().ApiGateway.GWPubKey
	if cc.AuthServer().ApiGateway.AutoRegister {
		gw, err := apigw.NewApiGw(cc.AuthServer().Esb, cc.AuthServer().ApiGateway)
		if err != nil {
			return err
		}

		result, err := gw.GetApigwPublicKey(cc.AuthServer().ApiGateway.Name)
		if err != nil {
			return err
		}
		if result.Code != 0 && result.Data.PublicKey == "" {
			return fmt.Errorf("get the gateway public key failed, err: %s", result.Message)
		}
		s.pubKey = result.Data.PublicKey
	}

	return nil
}

// Handler return service's handler.
func (s *Service) Handler() (http.Handler, error) {
	if s.gateway == nil {
		return nil, errors.New("gateway is nil")
	}

	return s.gateway.handler(), nil
}

// nolint: funlen
func newClientSet(sd serviced.Discover, tls cc.TLSConfig, iamSettings cc.IAM, disableAuth bool) (
	*ClientSet, error) {

	logs.Infof("start initialize the client set.")

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
			return nil, fmt.Errorf("init etcd tls config failed, err: %v", err)
		}

		cred := credentials.NewTLS(tlsC)
		opts = append(opts, grpc.WithTransportCredentials(cred))
	}

	// connect data service.
	dsConn, err := grpc.Dial(serviced.GrpcServiceDiscoveryName(cc.DataServiceName), opts...)
	if err != nil {
		return nil, fmt.Errorf("dial data service failed, err: %v", err)
	}
	ds := pbds.NewDataClient(dsConn)
	logs.Infof("initialize data service client success.")

	tlsConfig := new(tools.TLSConfig)
	if iamSettings.TLS.Enable() {
		tlsConfig = &tools.TLSConfig{
			InsecureSkipVerify: iamSettings.TLS.InsecureSkipVerify,
			CertFile:           iamSettings.TLS.CertFile,
			KeyFile:            iamSettings.TLS.KeyFile,
			CAFile:             iamSettings.TLS.CAFile,
			Password:           iamSettings.TLS.Password,
		}
	}
	cfg := &client.Config{
		Address:   []string{iamSettings.APIURL},
		AppCode:   iamSettings.AppCode,
		AppSecret: iamSettings.AppSecret,
		SystemID:  sys.SystemIDBSCP,
		TLS:       tlsConfig,
	}
	iamCli, err := client.NewClient(cfg, metrics.Register())
	if err != nil {
		return nil, err
	}

	iamSys, err := sys.NewSys(iamCli)
	if err != nil {
		return nil, fmt.Errorf("new iam sys failed, err: %v", err)
	}
	logs.Infof("initialize iam sys success.")

	// initialize iam auth sdk
	iamLgc, err := iam.NewIAM(ds, iamSys, disableAuth)
	if err != nil {
		return nil, fmt.Errorf("new iam logics failed, err: %v", err)
	}

	authSdk, err := pkgauth.NewAuth(iamCli, iamLgc)
	if err != nil {
		return nil, fmt.Errorf("new iam auth sdk failed, err: %v", err)
	}
	logs.Infof("initialize iam auth sdk success.")

	esbSetting := cc.AuthServer().Esb
	esbCli, err := esbcli.NewClient(&esbSetting, metrics.Register())
	if err != nil {
		return nil, err
	}
	cmdbConfig := cc.G().CMDB
	cmdb, err := bkcmdb.New(&cmdbConfig, esbCli)
	if err != nil {
		return nil, err
	}

	log := &logrus.Logger{
		Out:          os.Stderr,
		Formatter:    new(logrus.TextFormatter),
		Hooks:        make(logrus.LevelHooks),
		Level:        logrus.DebugLevel,
		ExitFunc:     os.Exit,
		ReportCaller: false,
	}
	bkiamlogger.SetLogger(log)

	cs := &ClientSet{
		DS:   ds,
		sys:  iamSys,
		auth: authSdk,
		Esb:  esbCli,
		cmdb: cmdb,
		iam: &auth.Iam{
			SystemID:  sys.SystemIDBSCP,
			AppCode:   iamSettings.AppCode,
			AppSecret: iamSettings.AppSecret,
			APIURL:    iamSettings.APIURL,
		},
	}

	// V4 客户端只在启用 v4 时构造，v3 模式下保持为 nil，避免为未使用的版本要求配置齐全。
	if iamSettings.IsV4() {
		cs.iamV4, err = newIAMV4Client(iamSettings.V4)
		if err != nil {
			return nil, err
		}
		logs.Infof("initialize iam v4 gateway client success, system: %s", iamSettings.V4.SystemID)
	}

	logs.Infof("initialize the client set success.")
	return cs, nil
}

// newIAMV4Client 构造 bkiam 网关客户端。
func newIAMV4Client(settings cc.IAMV4) (*clientv4.Client, error) {
	cli, err := clientv4.NewClient(&clientv4.Config{
		GatewayURL: settings.GatewayURL,
		SystemID:   settings.SystemID,
		AppCode:    settings.AppCode,
		AppSecret:  settings.AppSecret,
	})
	if err != nil {
		return nil, fmt.Errorf("new iam v4 gateway client failed, err: %v", err)
	}

	return cli, nil
}

// ClientSet defines configure server's all the depends api client.
type ClientSet struct {
	// data service's sys api
	DS  pbds.DataClient
	sys *sys.Sys
	// auth related operate.
	auth pkgauth.Authorizer
	// Esb Esb client api
	Esb  esbcli.Client
	iam  *auth.Iam
	cmdb bkcmdb.Service
	// iamV4 bkiam 网关客户端，仅当 iam.version 为 v4 时非空
	iamV4 *clientv4.Client
}

// PullResource 是权限中心拉取资源实例的回调入口，按运行时配置分发到 V3 或 V4 的实现。
// 两个版本的协议不兼容：分页字段、requires 的位置、_bk_iam_path_ 的类型以及响应包装都不同，
// 因此各自独立实现，这里只做路由。
func (s *Service) PullResource(ctx context.Context, req *pbas.PullResourceReq) (*structpb.Struct, error) {
	if s.iamSettings.IsV4() {
		return s.pullResourceV4(ctx, req)
	}

	return s.iam.PullResource(ctx, req)
}

// pullResourceV4 处理 V4 协议的资源回调。
func (s *Service) pullResourceV4(ctx context.Context, req *pbas.PullResourceReq) (
	*structpb.Struct, error) {

	kt := kit.FromGrpcContext(ctx)

	if s.iamV4Lgc == nil {
		return nil, errors.New("iam v4 callback logic is not initialized")
	}

	v4Req, err := iamv4.ParsePullResourceReq(req)
	if err != nil {
		logs.Errorf("parse iam v4 pull resource request failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	data, err := s.iamV4Lgc.PullResource(kt, v4Req)
	if err != nil {
		logs.Errorf("iam v4 pull resource failed, type: %s, method: %s, err: %v, rid: %s",
			v4Req.Type, v4Req.Method, err, kt.Rid)
		return nil, err
	}

	return iamv4.MarshalResp(data)
}

// InitAuthCenter init auth center's auth model.
func (s *Service) InitAuthCenter(ctx context.Context, req *pbas.InitAuthCenterReq) (*pbas.InitAuthCenterResp, error) {
	return s.initial.InitAuthCenter(ctx, req)
}

// GetAuthConf get auth login conf
func (s *Service) GetAuthConf(_ context.Context,
	_ *pbas.GetAuthConfReq) (*pbas.GetAuthConfResp, error) {

	resp := &pbas.GetAuthConfResp{
		LoginAuth: &pbas.LoginAuth{
			Host:      cc.AuthServer().LoginAuth.Host,
			InnerHost: cc.AuthServer().LoginAuth.InnerHost,
			Provider:  cc.AuthServer().LoginAuth.Provider,
			GwPubkey:  s.pubKey,
			UseEsb:    false,
		},
		Esb: &pbas.ESB{
			Endpoints: cc.AuthServer().Esb.Endpoints,
			AppCode:   cc.AuthServer().Esb.AppCode,
			AppSecret: cc.AuthServer().Esb.AppSecret,
			User:      cc.AuthServer().Esb.User,
			Tls: &pbas.TLS{
				InsecureSkipVerify: cc.AuthServer().Esb.TLS.InsecureSkipVerify,
				CertFile:           cc.AuthServer().Esb.TLS.CertFile,
				KeyFile:            cc.AuthServer().Esb.TLS.KeyFile,
				CaFile:             cc.AuthServer().Esb.TLS.CAFile,
				Password:           cc.AuthServer().Esb.TLS.Password,
			},
		},
		Cmdb: &pbas.CMDB{
			Host:       cc.G().CMDB.Host,
			AppCode:    cc.G().CMDB.AppCode,
			AppSecret:  cc.G().CMDB.AppSecret,
			BkUserName: cc.G().CMDB.BkUserName,
			UseEsb:     cc.G().CMDB.UseEsb,
		},
	}
	return resp, nil
}

// AuthorizeBatch authorize resource batch.
func (s *Service) AuthorizeBatch(ctx context.Context, req *pbas.AuthorizeBatchReq) (*pbas.AuthorizeBatchResp, error) {
	return s.auth.AuthorizeBatch(ctx, req)
}

// GetPermissionToApply get iam permission to apply.
func (s *Service) GetPermissionToApply(ctx context.Context, req *pbas.GetPermissionToApplyReq) (
	*pbas.GetPermissionToApplyResp, error) {

	return s.auth.GetPermissionToApply(ctx, req)
}

// GrantResourceCreatorAction GetPermissionToApply get iam permission to apply.
func (s *Service) GrantResourceCreatorAction(ctx context.Context, req *pbas.
	GrantResourceCreatorActionReq) (*base.EmptyResp, error) {

	err := s.auth.GrantResourceCreatorAction(ctx, pbas.GrantResourceCreatorAction(req))
	return nil, err

}

// CheckPermission grpc check permission
func (s *Service) CheckPermission(ctx context.Context, req *pbas.CheckPermissionReq) (
	*pbas.CheckPermissionResp, error) {
	kt := kit.FromGrpcContext(ctx)

	resp := &pbas.CheckPermissionResp{
		IsAllowed: false,
		ApplyUrl:  "",
		Resources: []*pbas.BasicDetail{},
	}

	userInfo := &meta.UserInfo{UserName: kt.User}
	abReq := &pbas.AuthorizeBatchReq{
		User:      pbas.PbUserInfo(userInfo),
		Resources: req.Resources,
	}

	abResp, err := s.AuthorizeBatch(kt.RpcCtx(), abReq)
	if err != nil {
		logs.Errorf("authorize failed, req: %#v, err: %v, rid: %s", req, err, kt.Rid)
		return nil, err
	}

	authorized := true
	for _, decision := range abResp.Decisions {
		if !decision.Authorized {
			authorized = false
			break
		}
	}

	if authorized {
		resp.IsAllowed = true
		return resp, nil
	}

	gpReq := &pbas.GetPermissionToApplyReq{
		Resources: req.Resources,
	}

	permResp, err := s.GetPermissionToApply(kt.RpcCtx(), gpReq)
	if err != nil {
		logs.Errorf("get permission to apply failed, req: %#v, err: %v, rid: %s", req, err, kt.Rid)
		return nil, errf.New(errf.DoAuthorizeFailed, "get permission to apply failed")
	}
	resp.ApplyUrl = permResp.ApplyUrl
	for _, action := range permResp.Permission.Actions {
		for _, resourceType := range action.RelatedResourceTypes {
			for _, instance := range resourceType.Instances {
				for _, i := range instance.Instances {
					if i.Type != resourceType.Type {
						continue
					}
					resp.Resources = append(resp.Resources, &pbas.BasicDetail{
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
	return resp, nil
}

// initLogicModule init logic module.
func (s *Service) initLogicModule() error {
	var err error

	s.initial, err = initial.NewInitial(s.client.sys, s.disableAuth)
	if err != nil {
		return err
	}

	s.iam, err = iam.NewIAM(s.client.DS, s.client.sys, s.disableAuth)
	if err != nil {
		return err
	}

	// V4 回调逻辑只在启用 v4 时装配。它代理 data-service 取服务、
	// 代理 space.Manager 取业务——后者是 V4 下 biz 改由 BSCP 自行提供实例数据的落点。
	if s.iamSettings.IsV4() {
		s.iamV4Lgc, err = iamv4.NewIAM(s.client.DS, s.spaceMgr)
		if err != nil {
			return err
		}
	}

	s.auth, err = auth.NewAuth(s.client.auth, s.client.DS, s.disableAuth, func(tenantID string) *bkiam.IAM {
		return s.client.iam.WithTenant(tenantID)
	},
		s.disableWriteOpt,
		s.spaceMgr)
	if err != nil {
		return err
	}

	// 启用 v4 后鉴权、申请链接与创建者授权都改走 V4，V3 SDK 不再参与。
	// 这是 V3/V4 切换的唯一开关点，业务侧与 internal/iam/auth 的 gRPC 契约不受影响。
	if s.iamSettings.IsV4() {
		v4Settings := s.iamSettings.V4
		if err = s.auth.WithIAMV4(s.client.iamV4, authv4.Config{
			CacheSize:   v4Settings.AuthCacheSize,
			CacheTTL:    time.Duration(v4Settings.AuthCacheTTLSeconds) * time.Second,
			Concurrency: v4Settings.AuthConcurrency,
		}); err != nil {
			return err
		}
		logs.Infof("iam v4 authorizer enabled, cache size: %d, cache ttl: %ds, concurrency: %d",
			v4Settings.AuthCacheSize, v4Settings.AuthCacheTTLSeconds, v4Settings.AuthConcurrency)
	}

	return nil
}

// GetUserInfo 获取用户信息
func (s *Service) GetUserInfo(ctx context.Context, req *pbas.UserCredentialReq) (*pbas.UserInfoResp, error) {
	token := req.GetToken()
	if token == "" {
		return nil, errors.New("token not provided")
	}

	conf := cc.AuthServer().LoginAuth
	authLoginClient := bkpaas.NewAuthLoginClient(&conf)

	// 多租户模式
	if cc.AuthServer().FeatureFlags.EnableMultiTenantMode {
		tenant, err := authLoginClient.GetTenantUserInfoByToken(ctx, token)
		if err != nil {
			if errors.Is(err, errf.ErrPermissionDenied) {
				return nil, status.New(codes.PermissionDenied, errf.GetErrMsg(err)).Err()
			}
			return nil, err
		}

		slog.Info("get user info success in MultiTenantMode", "username", tenant.BkUsername,
			"tenant_id", tenant.TenantID, "time_zone", tenant.TimeZone)
		return &pbas.UserInfoResp{
			Username:  tenant.BkUsername,
			AvatarUrl: "",
			TenantId:  tenant.TenantID,
			TimeZone:  tenant.TimeZone,
		}, nil

	}

	// 优先使用 InnerHost
	host := cc.AuthServer().LoginAuth.Host
	if cc.AuthServer().LoginAuth.InnerHost != "" {
		host = cc.AuthServer().LoginAuth.InnerHost
	}

	var (
		username string
		err      error
	)

	if cc.AuthServer().LoginAuth.UseESB && cc.AuthServer().LoginAuth.Provider != bkpaas.BKLoginProvider {
		username, err = s.client.Esb.BKLogin().IsLogin(ctx, token)
	} else {
		username, err = authLoginClient.GetUserInfoByToken(ctx, host, req.GetUid(), token)
	}

	if err != nil {
		if errors.Is(err, errf.ErrPermissionDenied) {
			return nil, status.New(codes.PermissionDenied, errf.GetErrMsg(err)).Err()
		}
		return nil, err
	}

	slog.Info("get user info success", "username", username)
	return &pbas.UserInfoResp{Username: username, AvatarUrl: ""}, nil
}

// ListUserSpaceAnnotation list user space permission annotations
func ListUserSpaceAnnotation(ctx context.Context, kt *kit.Kit, authorizer iamauth.Authorizer,
	msg proto.Message) (*webannotation.Annotation, error) {

	resp, ok := msg.(*pbas.ListUserSpaceResp)
	if !ok {
		return nil, nil
	}

	perms := map[string]webannotation.Perm{}
	authRes := make([]*meta.ResourceAttribute, 0, len(resp.GetItems()))
	for _, v := range resp.GetItems() {
		bID, _ := strconv.ParseInt(v.SpaceId, 10, 64)
		authRes = append(authRes, &meta.ResourceAttribute{
			Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource, ResourceID: uint32(bID)}, BizID: uint32(bID)},
		)

	}

	authResp, _, err := authorizer.AuthorizeDecision(kt, authRes...)
	if err != nil {
		return nil, err
	}

	for idx, v := range resp.GetItems() {
		perms[v.SpaceId] = webannotation.Perm{string(meta.FindBusinessResource): authResp[idx].Authorized}
	}

	return &webannotation.Annotation{Perms: perms}, nil
}

func init() {
	webannotation.Register(&pbas.ListUserSpaceResp{}, ListUserSpaceAnnotation)
	webannotation.Register(&pbcs.ListAppsResp{}, confsvc.ListAppsAnnotation)
	// 项目/环境列表无法逐项拦截，需要把逐项权限带回前端做置灰与申请引导。
	webannotation.Register(&pbcs.ListProjectsResp{}, confsvc.ListProjectsAnnotation)
	webannotation.Register(&pbcs.ListEnvironmentsResp{}, confsvc.ListEnvironmentsAnnotation)
}

// ListUserSpace 获取用户信息
func (s *Service) ListUserSpace(ctx context.Context, req *pbas.ListUserSpaceReq) (*pbas.ListUserSpaceResp, error) {
	kt := kit.FromGrpcContext(ctx)
	if kt.User == "" {
		err := basepb.InvalidArgumentsErr(&basepb.InvalidArgument{
			Field:   "kit.user",
			Message: "kit.user not found in metadata",
		})

		return nil, err
	}

	// 定期同步
	spaceList := s.spaceMgr.AllSpaces(ctx)

	items := make([]*pbas.Space, 0, len(spaceList))
	for _, space := range spaceList {
		items = append(items, &pbas.Space{
			SpaceId:       space.SpaceId,
			SpaceName:     space.SpaceName,
			SpaceTypeId:   space.SpaceTypeID,
			SpaceTypeName: space.SpaceTypeName,
			SpaceUid:      space.SpaceUid,
			SpaceEnName:   space.SpaceEnName,
		})
	}

	return &pbas.ListUserSpaceResp{Items: items}, nil
}

// QuerySpace 查询 space 信息
func (s *Service) QuerySpace(ctx context.Context, req *pbas.QuerySpaceReq) (*pbas.QuerySpaceResp, error) {
	uidList := req.GetSpaceUid()
	if len(uidList) == 0 {
		return &pbas.QuerySpaceResp{}, nil
	}

	spaceList, err := s.spaceMgr.QuerySpace(ctx, uidList)
	if err != nil {
		return nil, err
	}

	items := make([]*pbas.Space, 0, len(spaceList))
	for _, space := range spaceList {
		items = append(items, &pbas.Space{
			SpaceId:       space.SpaceId,
			SpaceName:     space.SpaceName,
			SpaceTypeId:   space.SpaceTypeID,
			SpaceTypeName: space.SpaceTypeName,
			SpaceUid:      space.SpaceUid,
		})
	}

	return &pbas.QuerySpaceResp{Items: items}, nil
}

// QuerySpaceByAppID 查询space
func (s *Service) QuerySpaceByAppID(ctx context.Context, req *pbas.QuerySpaceByAppIDReq) (*pbas.Space, error) {
	kt := kit.FromGrpcContext(ctx)
	appID := req.GetAppId()
	if appID == 0 {
		return nil, errors.New("app_id is required")
	}

	app, err := s.client.DS.GetAppByID(kt.RpcCtx(), &pbds.GetAppByIDReq{AppId: appID})
	if err != nil {
		return nil, err
	}

	resp := &pbas.Space{
		SpaceId:       strconv.Itoa(int(app.BizId)),
		SpaceTypeId:   space.BK_CMDB.ID,
		SpaceTypeName: space.BK_CMDB.Name,
	}
	return resp, nil
}

// IAMVerify implements pbas.AuthServer. 校验资源回调请求携带的 token。
// V3 与 V4 的系统 token 来自不同接口，按运行时配置取用对应的那个。
func (s *Service) IAMVerify(ctx context.Context, req *pbas.IAMVerifyReq) (*pbas.IAMVerifyResp, error) {
	kt := kit.FromGrpcContext(ctx)
	if iamToken.token != "" && time.Since(iamToken.tokenRefreshTime) <= time.Minute && req.GetToken() == iamToken.token {
		return &pbas.IAMVerifyResp{IsAuthorized: true}, nil
	}

	token, err := s.getSystemToken(kt)
	if err != nil {
		logs.Errorf("check request authorization get system token failed, error: %s, rid: %s", err.Error(), kt.Rid)
		return &pbas.IAMVerifyResp{IsAuthorized: false}, err
	}

	iamToken.token = token
	iamToken.tokenRefreshTime = time.Now()

	if req.GetToken() != iamToken.token {
		logs.Errorf("request token does not match the system token, rid: %s", kt.Rid)
		return &pbas.IAMVerifyResp{IsAuthorized: false}, errors.New("request password not match system token")
	}

	return &pbas.IAMVerifyResp{IsAuthorized: true}, nil
}

// VerifyEnv implements [pbas.AuthServer].
func (s *Service) VerifyEnv(ctx context.Context, req *pbas.VerifyEnvReq) (*pbas.VerifyEnvResp, error) {
	kt := kit.FromGrpcContext(ctx)
	bizID := req.GetBizId()
	if bizID == 0 {
		return nil, errors.New("biz id is required")
	}

	projectID := req.GetProjectId()
	if projectID == 0 {
		return nil, errors.New("project id is required")
	}

	envID := req.GetEnvId()
	if envID == 0 {
		return nil, errors.New("env id is required")
	}

	_, err := s.client.DS.GetEnvironment(kt.RpcCtx(), &pbds.GetEnvironmentReq{
		BizId:     bizID,
		ProjectId: projectID,
		EnvId:     envID,
	})

	if err != nil {
		return nil, err
	}

	return &pbas.VerifyEnvResp{Exists: true}, nil
}

// VerifyProject implements [pbas.AuthServer].
func (s *Service) VerifyProject(ctx context.Context, req *pbas.VerifyProjectReq) (*pbas.VerifyProjectResp, error) {
	kt := kit.FromGrpcContext(ctx)
	bizID := req.GetBizId()
	if bizID == 0 {
		return nil, errors.New("biz id is required")
	}

	projectID := req.GetProjectId()
	if projectID == 0 {
		return nil, errors.New("project id is required")
	}

	_, err := s.client.DS.GetProject(kt.RpcCtx(), &pbds.GetProjectReq{
		BizId:     bizID,
		ProjectId: projectID,
	})

	if err != nil {
		return nil, err
	}

	return &pbas.VerifyProjectResp{Exists: true}, nil
}

// getSystemToken 取当前启用版本的系统 token。
func (s *Service) getSystemToken(kt *kit.Kit) (string, error) {
	if !s.iamSettings.IsV4() {
		return s.gateway.iamSys.GetSystemToken(kt.Ctx)
	}

	if s.client.iamV4 == nil {
		return "", errors.New("iam v4 gateway client is not initialized")
	}

	return s.client.iamV4.GetSystemAuthToken(kt)
}
