// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package extauthz

import (
	"encoding/json"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/wrapperspb"

	routes "github.com/srinandan/envoy-router/server/routes"
	rpcstatus "google.golang.org/genproto/googleapis/rpc/status"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	auth "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	"github.com/gogo/googleapis/google/rpc"
	common "github.com/srinandan/sample-apps/common"
)

// inspired by https://github.com/salrashid123/envoy_external_authz/blob/master/authz_server/grpc_server.go

// Register registers
func (a *AuthorizationServer) Register(s *grpc.Server) {
	auth.RegisterAuthorizationServer(s, a)
}

// AuthorizationServer server
type AuthorizationServer struct{}

func (a *AuthorizationServer) Check(ctx context.Context, req *auth.CheckRequest) (*auth.CheckResponse, error) {
	common.Info.Println(">>> Authorization called check()")

	if req.Attributes != nil &&
		req.Attributes.Request != nil &&
		req.Attributes.Request.Http != nil &&
		req.Attributes.Request.Http.Headers != nil {
		common.Info.Printf(">>>> ExtAuthz_Request_headers: %v \n", req.Attributes.Request.Http.Headers)
	}

	if req.Attributes != nil && req.Attributes.ContextExtensions != nil {
		if ct, err := json.MarshalIndent(req.Attributes.ContextExtensions, "", "  "); err == nil {
			common.Info.Printf(">>>> Context Extensions: %s\n", string(ct))
		}
	}

	if req.Attributes != nil &&
		req.Attributes.Request != nil &&
		req.Attributes.Request.Http != nil {

		if req.Attributes.Request.Http.Body != "" {
			common.Info.Printf(">>>> Payload: %s\n", req.Attributes.Request.Http.Body)
		}

		if backend, prefix, found := routes.GetRoute(req.Attributes.Request.Http.Path); found {
			basepath := routes.ReplacePrefix(req.Attributes.Request.Http.Path, prefix)
			return checkResponse(backend, basepath), nil
		} else {
			return checkNotFoundResponse(), nil
		}

	}

	return checkNotFoundResponse(), nil
}

func checkNotFoundResponse() *auth.CheckResponse {
	common.Info.Println(">>> Authorization CheckResponse_NOTFOUND")
	return &auth.CheckResponse{
		Status: &rpcstatus.Status{
			Code: int32(rpc.NOT_FOUND),
		},
	}
}

func checkResponse(backend string, basepath string) *auth.CheckResponse {
	common.Info.Println(">>> Authorization CheckResponse_OkResponse")
	common.Info.Println(">>>> Selecting route ", backend, basepath)

	return &auth.CheckResponse{
		Status: &rpcstatus.Status{
			Code: int32(rpc.OK),
		},
		HttpResponse: &auth.CheckResponse_OkResponse{
			OkResponse: &auth.OkHttpResponse{
				Headers: []*corev3.HeaderValueOption{
					setHeader("host", backend, false),
					setHeader(":path", basepath, false),
				},
			},
		},
	}
}

func setHeader(name string, value string, append bool) *corev3.HeaderValueOption {
	header := &corev3.HeaderValue{
		Key:   name,
		Value: value,
	}

	return &corev3.HeaderValueOption{
		Header: header,
		Append: &wrapperspb.BoolValue{Value: append},
	}
}
