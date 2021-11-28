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

package main

import (
	"flag"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	extauthz "github.com/srinandan/envoy-router/server/extauthz"
	extproc "github.com/srinandan/envoy-router/server/extproc"
	routes "github.com/srinandan/envoy-router/server/routes"
	common "github.com/srinandan/sample-apps/common"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
)

func main() {
	var routeFile string

	//init logging
	common.InitLog()

	flag.StringVar(&routeFile, "routes", "routes.json", "A file containing routes")
	flag.Parse()

	if err := routes.ReadRoutesFile(routeFile); err != nil {
		common.Error.Println("unable to load routing table: ", err)
		os.Exit(1)
	}
	serve()
	select {}
}

func serve() {
	// gRPC server
	opts := []grpc.ServerOption{
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionAge: 10 * time.Minute,
		}),
		grpc.MaxConcurrentStreams(10),
	}

	grpcServer := grpc.NewServer(opts...)

	as := &extauthz.AuthorizationServer{}
	as.Register(grpcServer)

	ep := &extproc.ExternalProcessingServer{}
	ep.Register(grpcServer)

	// grpc health
	grpcHealth := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, grpcHealth)

	common.Info.Println("starting gRPC Server at ", common.GetgRPCPort())

	// grpc listener
	grpcListener, err := net.Listen("tcp", ":"+common.GetgRPCPort())
	if err != nil {
		panic(err)
	}

	go func() {
		if err := grpcServer.Serve(grpcListener); err != nil {
			common.Info.Printf("%s", err)
		}
	}()

	// watch for termination signals
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt)    // terminal
		signal.Notify(sigint, syscall.SIGTERM) // kubernetes
		sig := <-sigint
		common.Info.Printf("shutdown signal: %s\n", sig)
		signal.Stop(sigint)

		grpcServer.GracefulStop()

		common.Info.Println("shutdown complete")
		os.Exit(0)
	}()
}
