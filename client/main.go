// Copyright 2020 Google LLC
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

package main

import (
	"context"
	"fmt"
	"os"

	auth "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	"google.golang.org/grpc"
)

func getGRPCPort() string {
	port := os.Getenv("GRPC_PORT")
	if port == "" {
		return ":50051"
	}
	return port
}

func main() {

	ctx := context.Background()

	conn, err := grpc.Dial(getGRPCPort(), grpc.WithInsecure())
	if err != nil {
		fmt.Printf("did not connect: %v", err)
		return
	}

	authorizationClient := auth.NewAuthorizationClient(conn)

	callroute(ctx, authorizationClient, "/httpbin")
	callroute(ctx, authorizationClient, "/integrations")
	callroute(ctx, authorizationClient, "/notfound")
}

func callroute(ctx context.Context, authorizationClient auth.AuthorizationClient, route string) {

	checkRequest := auth.CheckRequest{}

	http := auth.AttributeContext_HttpRequest{}
	http.Path = route

	request := auth.AttributeContext_Request{}
	request.Http = &http

	attributes := auth.AttributeContext{}
	attributes.Request = &request

	checkRequest.Attributes = &attributes

	checkResponse, err := authorizationClient.Check(
		ctx,
		&checkRequest,
	)

	if err != nil {
		fmt.Printf("did not receive check response: %v", err)
		return
	}

	fmt.Printf("Response was %d\n", checkResponse.Status.Code)

	if response, ok := checkResponse.HttpResponse.(*auth.CheckResponse_DeniedResponse); ok {
		fmt.Println("Request was denied")
		fmt.Printf("%v\n", response.DeniedResponse)
	}

	if response, ok := checkResponse.HttpResponse.(*auth.CheckResponse_OkResponse); ok {
		fmt.Println("Request was ok, printing headers")
		fmt.Println(response.OkResponse.Headers)
	}

}
