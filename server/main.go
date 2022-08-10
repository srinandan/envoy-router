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
	"strconv"
	"syscall"
	"time"

	extauthz "github.com/srinandan/envoy-router/server/extauthz"
	extproc "github.com/srinandan/envoy-router/server/extproc"
	routes "github.com/srinandan/envoy-router/server/routes"
	token "github.com/srinandan/envoy-router/server/token"
	common "github.com/srinandan/sample-apps/common"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
)

//default file locations
const defaultServiceAccountFilePath = "/etc/secrets/sa.json"
const defaultRoutesFile = "/etc/routes/routes.json"

//default interval to obtain new access tokens
const interval = 25 * 60 //25 mins

//use this flag to disable auth
var disable_auth_envvar = os.Getenv("DISABLE_AUTH")
var disable_auth bool

func main() {
	var routeFile, key, cert, saFile string
	oauthtoken := token.AccessToken{}

	//init logging
	common.InitLog()

	flag.StringVar(&routeFile, "routes", defaultRoutesFile, "A file containing routes")
	flag.StringVar(&key, "key", "", "A file containing the private key")
	flag.StringVar(&cert, "cert", "", "A file containing the public key key")
	flag.StringVar(&saFile, "sa", "", "GCP Service Account JSON file")
	flag.Parse()

	if err := routes.ReadRoutesFile(routeFile); err != nil {
		common.Error.Printf("unable to load routing table %s: %v\n", routeFile, err)
	}

	if (key != "" && cert == "") || (key == "" && cert != "") {
		common.Error.Println("both key and cert must be specified")
		os.Exit(1)
	}

	if saFile != "" {
		token.SetServiceAccountFilePath(saFile)
	} else {
		token.SetServiceAccountFilePath(defaultServiceAccountFilePath)
	}

	if disable_auth_envvar != "" {
		disable_auth, _ = strconv.ParseBool(disable_auth_envvar)
	}

	if !disable_auth {
		if err := oauthtoken.ObtainAccessToken(); err != nil {
			common.Error.Println(err)
		}
	}

	serve(key, cert, &oauthtoken)
	select {}
}

func serve(key string, cert string, oauthtoken *token.AccessToken) {
	// gRPC server
	opts := []grpc.ServerOption{
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionAge: 10 * time.Minute,
		}),
		grpc.MaxConcurrentStreams(10),
		grpc.StreamInterceptor(grpc_prometheus.StreamServerInterceptor),
		grpc.UnaryInterceptor(grpc_prometheus.UnaryServerInterceptor),
	}

	if cert != "" && key != "" {
		creds, err := credentials.NewServerTLSFromFile(cert, key)
		if err != nil {
			panic(err)
		}
		opts = append(opts, grpc.Creds(creds))
	}

	grpcServer := grpc.NewServer(opts...)
	grpc_prometheus.Register(grpcServer)

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

	if !disable_auth {
		//obtain a new token every 25 mins
		stop := token.Every(interval*time.Second, func(time.Time) bool {
			common.Info.Println("obtaining a new access token")
			if err := oauthtoken.ObtainAccessToken(); err != nil {
				common.Error.Println(err)
			}
			return true
		})

		<-stop
	}
}
