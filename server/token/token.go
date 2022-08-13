// Copyright 2022 Google LLC
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

package token

import (
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwt"

	common "github.com/srinandan/sample-apps/common"
)

type serviceAccount struct {
	Type                string `json:"type,omitempty"`
	ProjectID           string `json:"project_id,omitempty"`
	PrivateKeyID        string `json:"private_key_id,omitempty"`
	PrivateKey          string `json:"private_key,omitempty"`
	ClientEmail         string `json:"client_email,omitempty"`
	ClientID            string `json:"client_id,omitempty"`
	AuthURI             string `json:"auth_uri,omitempty"`
	TokenURI            string `json:"token_uri,omitempty"`
	AuthProviderCertURL string `json:"auth_provider_x509_cert_url,omitempty"`
	ClientCertURL       string `json:"client_x509_cert_url,omitempty"`
}

type AccessToken struct {
	token string
	sync.Mutex
}

var account = serviceAccount{}

const tokenUri = "https://www.googleapis.com/oauth2/v4/token"

var serviceAccountPath string

func getPrivateKey(privateKey string) (interface{}, error) {
	pemPrivateKey := fmt.Sprintf("%v", privateKey)
	block, _ := pem.Decode([]byte(pemPrivateKey))
	if block == nil {
		return nil, fmt.Errorf("Invalid format of private key")
	}
	privKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		common.Error.Println("error parsing Private Key: ", err)
		return nil, err
	}
	return privKey, nil
}

func generateJWT(privateKey string) (string, error) {

	const scope = "https://www.googleapis.com/auth/cloud-platform"

	privKey, err := getPrivateKey(privateKey)

	if err != nil {
		return "", err
	}

	now := time.Now()
	token := jwt.New()

	//Google OAuth takes aud as a string, not array
	jwt.Settings(jwt.WithFlattenAudience(true))

	_ = token.Set("aud", tokenUri)
	_ = token.Set(jwt.IssuerKey, getServiceAccountProperty("ClientEmail"))
	_ = token.Set("scope", scope)
	_ = token.Set(jwt.IssuedAtKey, now.Unix())
	_ = token.Set(jwt.ExpirationKey, now.Unix())

	payload, err := jwt.Sign(token, jwt.WithKey(jwa.RS256, privKey))
	if err != nil {
		common.Error.Println("error parsing Private Key: ", err)
		return "", err
	}
	common.Info.Println("jwt token : ", string(payload))
	return string(payload), nil
}

//generateAccessToken generates a Google OAuth access token from a service account
func generateAccessToken(privateKey string) (string, error) {

	const grantType = "urn:ietf:params:oauth:grant-type:jwt-bearer"
	var respBody []byte

	//oAuthAccessToken is a structure to hold OAuth response
	type oAuthAccessToken struct {
		AccessToken string `json:"access_token,omitempty"`
		ExpiresIn   int    `json:"expires_in,omitempty"`
		TokenType   string `json:"token_type,omitempty"`
	}

	token, err := generateJWT(privateKey)

	if err != nil {
		return "", err
	}

	form := url.Values{}
	form.Add("grant_type", grantType)
	form.Add("assertion", token)

	client := &http.Client{}
	req, err := http.NewRequest("POST", tokenUri, strings.NewReader(form.Encode()))
	if err != nil {
		common.Error.Println("error in client: ", err)
		return "", err
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Content-Length", strconv.Itoa(len(form.Encode())))

	resp, err := client.Do(req)

	if err != nil {
		common.Error.Println("failed to generate oauth token: ", err)
		return "", err
	}

	if resp != nil {
		defer resp.Body.Close()
	}

	if resp == nil {
		common.Error.Println("error in response: Response was null")
		return "", errors.New("error in response: Response was null")
	}

	respBody, err = ioutil.ReadAll(resp.Body)
	common.Info.Printf("Response: %s\n", string(respBody))

	if err != nil {
		common.Error.Println("error in response: ", err)
		return "", fmt.Errorf("error in response: %v", err)
	} else if resp.StatusCode > 399 {
		common.Error.Printf("status code %d, error in response: %s\n", resp.StatusCode, string(respBody))
		return "", fmt.Errorf("status code %d, error in response: %s\n", resp.StatusCode, string(respBody))
	}

	accessToken := oAuthAccessToken{}
	if err = json.Unmarshal(respBody, &accessToken); err != nil {
		return "", err
	}

	common.Info.Println("access token object: ", accessToken)

	return accessToken.AccessToken, nil
}

func readServiceAccount() error {
	content, err := ioutil.ReadFile(serviceAccountPath)
	if err != nil {
		return err
	}

	err = json.Unmarshal(content, &account)
	if err != nil {
		return err
	}
	return nil
}

func getServiceAccountProperty(key string) (value string) {
	r := reflect.ValueOf(&account)
	field := reflect.Indirect(r).FieldByName(key)
	return field.String()
}

func checkAccessToken() bool {

	const tokenInfo = "https://oauth2.googleapis.com/tokeninfo"

	a := AccessToken{}

	u, _ := url.Parse(tokenInfo)
	q := u.Query()
	q.Set("access_token", a.GetAccessToken())
	u.RawQuery = q.Encode()

	client := &http.Client{}

	log.Println("Connecting to : ", u.String())
	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		log.Println("error in client:", err)
		return false
	}

	resp, err := client.Do(req)
	if err != nil {
		common.Error.Println("error connecting to token endpoint: ", err)
		return false
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		common.Error.Println("token info error: ", err)
		return false
	} else if resp.StatusCode != 200 {
		common.Error.Println("token expired: ", string(body))
		return false
	}
	common.Info.Println("Response: ", string(body))
	common.Info.Println("Reusing the cached token: ", a.GetAccessToken())
	return true
}

func SetServiceAccountFilePath(saFile string) {
	serviceAccountPath = saFile
}

func (a *AccessToken) GetAccessToken() string {
	return a.token
}

//ObtainAccessToken will generate a new one
func (a *AccessToken) ObtainAccessToken() (err error) {

	var token string

	if err = readServiceAccount(); err != nil { // Handle errors reading the config file
		return fmt.Errorf("error reading SA file: %s", err)
	}

	privateKey := getServiceAccountProperty("PrivateKey")
	if privateKey == "" {
		return fmt.Errorf("private key missing in the service account")
	}
	if getServiceAccountProperty("ClientEmail") == "" {
		return fmt.Errorf("client email missing in the service account")
	}
	if token, err = generateAccessToken(privateKey); err != nil {
		return fmt.Errorf("fatal error generating access token: %s", err)
	}

	a.Lock()
	defer a.Unlock()
	a.token = token
	return nil
}

func Every(duration time.Duration, work func(time.Time) bool) chan bool {
	ticker := time.NewTicker(duration)
	stop := make(chan bool, 1)

	go func() {
		for {
			select {
			case time := <-ticker.C:
				if !work(time) {
					stop <- true
				}
			case <-stop:
				return
			}
		}
	}()

	return stop
}
