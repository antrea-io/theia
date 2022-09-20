// Copyright 2022 Antrea Authors
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
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	intelligenceinstall "antrea.io/theia/pkg/apis/intelligence/install"
	intelligence "antrea.io/theia/pkg/apis/intelligence/v1alpha1"
	"antrea.io/theia/pkg/apiserver/certificate"
	"antrea.io/theia/pkg/apiserver/registry/intelligence/networkpolicyrecommendation"
	"antrea.io/theia/pkg/querier"
)

const (
	CertDir           = "/var/run/theia/theia-manager-tls"
	SelfSignedCertDir = "/var/run/theia/theia-manager-self-signed"
	Name              = "theia-manager-api"
	// AuthenticationTimeout specifies a time limit for requests made by the authorization webhook client
	// The default value (10 seconds) is not long enough as defined in
	// https://pkg.go.dev/k8s.io/apiserver@v0.21.0/pkg/server/options#NewDelegatingAuthenticationOptions
	// A value of zero means no timeout.
	AuthenticationTimeout = 0
)

var (
	// Scheme defines methods for serializing and deserializing API objects.
	scheme = runtime.NewScheme()
	// Codecs provides methods for retrieving codecs and serializers for specific
	// versions and content types.
	Codecs = serializer.NewCodecFactory(scheme)
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

// ExtraConfig holds custom apiserver config.
type ExtraConfig struct {
	k8sClient               kubernetes.Interface
	caCertController        *certificate.CACertController
	npRecommendationQuerier querier.NPRecommendationQuerier
}

// Config defines the config for Theia manager apiserver.
type Config struct {
	genericConfig *genericapiserver.Config
	extraConfig   ExtraConfig
}

type TheiaManagerAPIServer struct {
	GenericAPIServer        *genericapiserver.GenericAPIServer
	caCertController        *certificate.CACertController
	NPRecommendationQuerier querier.NPRecommendationQuerier
}

func (s *TheiaManagerAPIServer) Run(ctx context.Context) error {
	// Make sure CACertController runs once to publish the CA cert before starting APIServer.
	if err := s.caCertController.RunOnce(ctx); err != nil {
		klog.Warningf("caCertController RunOnce failed: %v", err)
	}
	go s.caCertController.Run(ctx, 1)
	return s.GenericAPIServer.PrepareRun().Run(ctx.Done())
}

func NewConfig(
	genericConfig *genericapiserver.Config,
	k8sClient kubernetes.Interface,
	caCertController *certificate.CACertController,
	npRecommendationQuerier querier.NPRecommendationQuerier) *Config {
	return &Config{
		genericConfig: genericConfig,
		extraConfig: ExtraConfig{
			k8sClient:               k8sClient,
			caCertController:        caCertController,
			npRecommendationQuerier: npRecommendationQuerier,
		},
	}
}

func installAPIGroup(s *TheiaManagerAPIServer) error {
	npRecommendationStorage := networkpolicyrecommendation.NewREST(s.NPRecommendationQuerier)
	intelligenceGroup := genericapiserver.NewDefaultAPIGroupInfo(intelligence.GroupName, scheme, parameterCodec, Codecs)
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

func (c Config) New() (*TheiaManagerAPIServer, error) {
	completedServerCfg := c.genericConfig.Complete(nil)
	s, err := completedServerCfg.New(Name, genericapiserver.NewEmptyDelegate())
	if err != nil {
		return nil, err
	}
	apiServer := &TheiaManagerAPIServer{
		GenericAPIServer:        s,
		caCertController:        c.extraConfig.caCertController,
		NPRecommendationQuerier: c.extraConfig.npRecommendationQuerier}
	if err := installAPIGroup(apiServer); err != nil {
		return nil, err
	}
	return apiServer, nil
}

func DefaultCAConfig() *certificate.CAConfig {
	return &certificate.CAConfig{
		CAConfigMapName:   certificate.TheiaCAConfigMapName,
		CertDir:           CertDir,
		SelfSignedCertDir: SelfSignedCertDir,
		CertReadyTimeout:  2 * time.Minute,
		MaxRotateDuration: time.Hour * (24 * 365),
		ServiceName:       certificate.TheiaServiceName,
		PairName:          Name,
	}
}
