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
	"fmt"
	"net"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/TencentBlueKing/bk-bscp/internal/dal/bedis"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	"github.com/TencentBlueKing/bk-bscp/internal/runtime/grpcgw"
	"github.com/TencentBlueKing/bk-bscp/internal/runtime/handler"
	"github.com/TencentBlueKing/bk-bscp/internal/serviced"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	pbcs "github.com/TencentBlueKing/bk-bscp/pkg/protocol/cache-service"
	"github.com/TencentBlueKing/bk-bscp/pkg/tools"
)

// gateway auth server's grpc-gateway.
type gateway struct {
	mux   *runtime.ServeMux
	dao   dao.Set
	bs    bedis.Client
	state serviced.State
}

// newGateway create new CacheService's grpc-gateway.
func newGateway(st serviced.State, dao dao.Set, bs bedis.Client) (*gateway, error) {
	mux, err := newCacheServiceMux()
	if err != nil {
		return nil, err
	}

	g := &gateway{
		state: st,
		dao:   dao,
		bs:    bs,
		mux:   mux,
	}

	return g, nil
}

// handler return gateway handler.
func (g *gateway) handler() http.Handler {
	r := chi.NewRouter()
	r.Use(handler.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/-/healthy", g.HealthyHandler)
	r.Get("/-/ready", g.ReadyHandler)
	r.Get("/healthz", g.Healthz)

	r.Mount("/", handler.RegisterCommonHandler())
	return r
}

// newCacheServiceMux new CacheService mux.
func newCacheServiceMux() (*runtime.ServeMux, error) {
	opts := make([]grpc.DialOption, 0)

	network := cc.CacheService().Network
	tls := network.TLS
	if !tls.Enable() {
		// dial without ssl
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		// dial with ssl.
		tlsC, err := tools.ClientTLSConfVerify(tls.InsecureSkipVerify, tls.CAFile, tls.CertFile, tls.KeyFile,
			tls.Password)
		if err != nil {
			return nil, fmt.Errorf("init grpc tls config failed, err: %v", err)
		}

		cred := credentials.NewTLS(tlsC)
		opts = append(opts, grpc.WithTransportCredentials(cred))
	}

	addr := net.JoinHostPort(network.BindIP, strconv.Itoa(int(network.RpcPort)))
	conn, err := grpc.Dial(addr, opts...)
	if err != nil {
		logs.Errorf("dial CacheService failed, err: %v", err)
		return nil, err
	}

	// new grpc mux.
	mux := newGrpcMux()

	// register client to mux.
	if err = pbcs.RegisterCacheHandler(context.Background(), mux, conn); err != nil {
		logs.Errorf("register CacheService handler client failed, err: %v", err)
		return nil, err
	}

	return mux, nil
}

// newGrpcMux new grpc mux that has some processing of built-in http request to grpc request.
func newGrpcMux() *runtime.ServeMux {
	return runtime.NewServeMux(grpcgw.MetadataOpt, grpcgw.JsonMarshalerOpt)
}
