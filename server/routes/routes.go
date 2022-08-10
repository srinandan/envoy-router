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

package routes

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"

	common "github.com/srinandan/sample-apps/common"
)

type Auth uint8

const (
	OFF Auth = iota
	ACCESS_TOKEN
	OIDC_TOKEN
)

type routerule struct {
	Name           string `json:"name,omitempty"`
	Backend        string `json:"backend,omitempty"`
	Prefix         string `json:"prefix,omitempty"`
	Authentication Auth   `json:"authentication,omitempty"`
}

type routeinfo struct {
	RouteRules []routerule `json:"routerules,omitempty"`
}

var routeInfo = routeinfo{}

func ReadRoutesFile(routeFile string) error {
	routeListBytes, err := ioutil.ReadFile(routeFile)
	if err != nil {
		return err
	}

	if err = json.Unmarshal(routeListBytes, &routeInfo); err != nil {
		return err
	}

	if len(routeInfo.RouteRules) < 1 {
		return fmt.Errorf("routing table must have at least one route rule")
	}

	return nil
}

func GetRoute(basePath string) (backend string, prefix string, a Auth, notFound bool) {
	common.Info.Printf(">>>>> basepath %s", basePath)

	for _, routeRule := range routeInfo.RouteRules {
		matchStr := "^" + routeRule.Prefix + "(/[^/]+)*/?"
		if ok, _ := regexp.MatchString(matchStr, basePath); ok {
			common.Info.Printf(">>>>> basepath found. authentication is %d\n", routeRule.Authentication)
			return routeRule.Backend, routeRule.Prefix, routeRule.Authentication, true
		}
	}
	common.Info.Printf(">>>>> basepath not found\n")
	return "", "", OFF, false
}

func ReplacePrefix(basePath string, prefix string) string {
	common.Info.Printf(">>>>> replace %s with %s", basePath, strings.Replace(basePath, prefix, "", 1))
	return strings.Replace(basePath, prefix, "", 1)
}
