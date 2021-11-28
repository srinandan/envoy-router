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

package extproc

import (
	"io"
	"os"
	"strconv"

	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	ext_proc "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_proc/v3"
	proc "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"github.com/golang/protobuf/ptypes/wrappers"
	routes "github.com/srinandan/envoy-router/server/routes"
	common "github.com/srinandan/sample-apps/common"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const statusField = ":status"

var routing = os.Getenv("ENABLE_ROUTING")

// Register registers
func (e *ExternalProcessingServer) Register(s *grpc.Server) {
	proc.RegisterExternalProcessorServer(s, e)
}

// ExternalProcessingServer server
type ExternalProcessingServer struct{}

func (e *ExternalProcessingServer) Process(srv proc.ExternalProcessor_ProcessServer) error {
	var resp *proc.ProcessingResponse

	ctx := srv.Context()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		req, err := srv.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return status.Errorf(codes.Unknown, "cannot receive stream request: %v", err)
		}

		switch v := req.Request.(type) {
		case *proc.ProcessingRequest_RequestHeaders:
			resp = processRequestHeaders(v)
		case *proc.ProcessingRequest_RequestBody:
			resp = processRequestBody(v)
		case *proc.ProcessingRequest_ResponseHeaders:
			resp = processResponseHeaders(v)
		case *proc.ProcessingRequest_ResponseBody:
			resp = processResponseBody(v)
		default:
			common.Error.Printf("Unknown Request type %v\n", v)
		}
		if err := srv.Send(resp); err != nil {
			common.Error.Printf("send error %v", err)
		}
	}
}

func processResponseHeaders(headers *proc.ProcessingRequest_ResponseHeaders) *proc.ProcessingResponse {
	common.Info.Printf("ProcessingRequest_ResponseHeaders %v \n", headers)
	resp := &proc.ProcessingResponse{}
	var status int

	for _, header := range headers.ResponseHeaders.Headers.Headers {
		if header.Key == statusField {
			status, _ = strconv.Atoi(header.Value)
		}
	}

	if status < 300 { //successful response from the upstream
		responseHeaders := &proc.HeadersResponse{
			Response: &proc.CommonResponse{
				HeaderMutation: &proc.HeaderMutation{
					SetHeaders: []*core.HeaderValueOption{
						//add a sample header
						setHeader("x-apigee-response-ext_proc", "test-value", false),
					},
				},
				Status: proc.CommonResponse_CONTINUE,
			},
		}

		resp.Response = &proc.ProcessingResponse_ResponseHeaders{
			ResponseHeaders: responseHeaders,
		}

		resp.ModeOverride = &ext_proc.ProcessingMode{
			ResponseHeaderMode: ext_proc.ProcessingMode_SEND,
		}

	} else {
		common.Info.Printf("Error from upstream. Status %d\n", status)
	}
	return resp
}

func processRequestHeaders(headers *proc.ProcessingRequest_RequestHeaders) *proc.ProcessingResponse {
	common.Info.Printf("ProcessingRequest_RequestHeaders %v \n", headers)
	resp := &proc.ProcessingResponse{}
	var path string

	for _, header := range headers.RequestHeaders.Headers.Headers {
		if header.Key == ":path" {
			path = header.Value
			break
		}
	}

	if routing == "true" {
		if backend, prefix, found := routes.GetRoute(path); found {
			basepath := routes.ReplacePrefix(path, prefix)
			requestHeaders := &proc.HeadersResponse{
				Response: &proc.CommonResponse{
					HeaderMutation: &proc.HeaderMutation{
						SetHeaders: []*core.HeaderValueOption{
							// at the time of writing this, host is not modifiable from ext_proc
							// https://github.com/envoyproxy/envoy/blob/main/source/extensions/filters/http/ext_proc/mutation_utils.cc#L128
							// this is the warning received in the logs:
							// [2021-11-28 16:43:04.339][671420][debug][filter] [source/extensions/filters/http/ext_proc/mutation_utils.cc:63] Ignorning improper attempt to set header host
							setHeader("host", backend, false),
							setHeader(":path", basepath, false),
						},
					},
					Status:          proc.CommonResponse_CONTINUE,
					ClearRouteCache: true,
				},
			}
			resp.Response = &proc.ProcessingResponse_RequestHeaders{
				RequestHeaders: requestHeaders,
			}
			resp.ModeOverride = &ext_proc.ProcessingMode{
				RequestHeaderMode: ext_proc.ProcessingMode_SEND,
			}
		}
	}

	return resp
}

func processRequestBody(body *proc.ProcessingRequest_RequestBody) *proc.ProcessingResponse {
	resp := &proc.ProcessingResponse{}
	common.Info.Printf("ProcessingRequest_RequestBody %v \n", body)
	return resp
}

func processResponseBody(body *proc.ProcessingRequest_ResponseBody) *proc.ProcessingResponse {
	resp := &proc.ProcessingResponse{}
	common.Info.Printf("ProcessingRequest_ResponseBody %v \n", body)
	return resp
}

func setHeader(name string, value string, append bool) *core.HeaderValueOption {
	header := &core.HeaderValue{}
	header.Key = name
	header.Value = value

	return &core.HeaderValueOption{
		Header: header,
		Append: &wrappers.BoolValue{Value: append},
	}
}
