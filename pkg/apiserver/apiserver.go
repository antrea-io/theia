// Copyright 2020 Antre2 Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package apiserver

import (
	"fmt"
	"net"
	"os"
	"path"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apiserver/pkg/authorization/authorizerfactory"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericoptions "k8s.io/apiserver/pkg/server/options"

	intelligenceinstall "antrea.io/theia/pkg/apis/intelligence/install"
	intelligence "antrea.io/theia/pkg/apis/intelligence/v1alpha1"
	"antrea.io/theia/pkg/apiserver/registry/intelligence/networkpolicyrecommendation"
	"antrea.io/theia/pkg/querier"
)

const (
	Name = "theia-manager-api"
	// authenticationTimeout specifies a time limit for requests made by the authorization webhook client
	// The default value (10 seconds) is not long enough as defined in
	// https://pkg.go.dev/k8s.io/apiserver@v0.21.0/pkg/server/options#NewDelegatingAuthenticationOptions
	// A value of zero means no timeout.
	authenticationTimeout = 0
)

var (
	// Scheme defines methods for serializing and deserializing API objects.
	scheme = runtime.NewScheme()
	// Codecs provides methods for retrieving codecs and serializers for specific
	// versions and content types.
	codecs = serializer.NewCodecFactory(scheme)
	// ParameterCodec defines methods for serializing and deserializing url values
	// to versioned API objects and back.
	parameterCodec = runtime.NewParameterCodec(scheme)
	// #nosec G101: false positive triggered by variable name which includes "token"
	TokenPath = "/var/run/antrea/apiserver/loopback-client-token"
)

func init() {
	intelligenceinstall.Install(scheme)
	metav1.AddToGroupVersion(scheme, schema.GroupVersion{Version: "v1"})
}

type theiaManagerAPIServer struct {
	GenericAPIServer        *genericapiserver.GenericAPIServer
	NPRecommendationQuerier querier.NPRecommendationQuerier
}

func (s *theiaManagerAPIServer) Run(stopCh <-chan struct{}) error {
	return s.GenericAPIServer.PrepareRun().Run(stopCh)
}

func newConfig(bindPort int) (*genericapiserver.CompletedConfig, error) {
	secureServing := genericoptions.NewSecureServingOptions().WithLoopback()
	authentication := genericoptions.NewDelegatingAuthenticationOptions()

	// Set the PairName but leave certificate directory blank to generate in-memory by default.
	secureServing.ServerCert.CertDirectory = ""
	secureServing.ServerCert.PairName = Name
	secureServing.BindAddress = net.IPv4zero
	secureServing.BindPort = bindPort

	authentication.WithRequestTimeout(authenticationTimeout)

	if err := secureServing.MaybeDefaultWithSelfSignedCerts("localhost", nil, []net.IP{net.ParseIP("127.0.0.1"), net.IPv6loopback}); err != nil {
		return nil, fmt.Errorf("error creating self-signed certificates: %v", err)
	}
	serverConfig := genericapiserver.NewConfig(codecs)
	if err := secureServing.ApplyTo(&serverConfig.SecureServing, &serverConfig.LoopbackClientConfig); err != nil {
		return nil, err
	}
	if err := authentication.ApplyTo(&serverConfig.Authentication, serverConfig.SecureServing, nil); err != nil {
		return nil, err
	}

	// TODO (wshaoquan): use native k8s role-based auth
	serverConfig.Authorization = genericapiserver.AuthorizationInfo{
		Authorizer: authorizerfactory.NewAlwaysAllowAuthorizer()}
	if err := os.MkdirAll(path.Dir(TokenPath), os.ModeDir); err != nil {
		return nil, fmt.Errorf("error when creating dirs of token file: %v", err)
	}
	if err := os.WriteFile(TokenPath, []byte(serverConfig.LoopbackClientConfig.BearerToken), 0600); err != nil {
		return nil, fmt.Errorf("error when writing loopback access token to file: %v", err)
	}

	completedServerCfg := serverConfig.Complete(nil)
	return &completedServerCfg, nil
}

func installAPIGroup(s *theiaManagerAPIServer) error {
	npRecommendationStorage := networkpolicyrecommendation.NewREST(s.NPRecommendationQuerier)
	intelligenceGroup := genericapiserver.NewDefaultAPIGroupInfo(intelligence.GroupName, scheme, parameterCodec, codecs)
	v1alpha1Storage := map[string]rest.Storage{}
	v1alpha1Storage["networkpolicyrecommendations"] = npRecommendationStorage
	intelligenceGroup.VersionedResourcesStorageMap["v1alpha1"] = v1alpha1Storage

	groups := []*genericapiserver.APIGroupInfo{&intelligenceGroup}

	for _, apiGroupInfo := range groups {
		if err := s.GenericAPIServer.InstallAPIGroup(apiGroupInfo); err != nil {
			return err
		}
	}
	return nil
}

func New(nprq querier.NPRecommendationQuerier, bindPort int, cipherSuites []uint16, tlsMinVersion uint16) (*theiaManagerAPIServer, error) {
	cfg, err := newConfig(bindPort)
	if err != nil {
		return nil, err
	}
	s, err := cfg.New(Name, genericapiserver.NewEmptyDelegate())
	if err != nil {
		return nil, err
	}
	s.SecureServingInfo.CipherSuites = cipherSuites
	s.SecureServingInfo.MinTLSVersion = tlsMinVersion
	apiServer := &theiaManagerAPIServer{GenericAPIServer: s, NPRecommendationQuerier: nprq}
	if err := installAPIGroup(apiServer); err != nil {
		return nil, err
	}
	return apiServer, nil
}
