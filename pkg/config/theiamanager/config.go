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

package config

type TheiaManagerConfig struct {
	// apiServer contains APIServer related configuration options.
	APIServer APIServerConfig `yaml:"apiServer,omitempty"`
}

type APIServerConfig struct {
	// APIPort is the port for the theia-manager APIServer to serve on.
	// Defaults to 11347.
	APIPort int `yaml:"apiPort,omitempty"`
	// Indicates whether to use auto-generated self-signed TLS certificate.
	// If false, a Secret named "theia-manager-tls" must be provided with the following keys:
	//   ca.crt: <CA certificate>
	//   tls.crt: <TLS certificate>
	//   tls.key: <TLS private key>
	// Defaults to true.
	SelfSignedCert *bool `yaml:"selfSignedCert,omitempty"`
	// Cipher suites to use.
	TLSCipherSuites string `yaml:"tlsCipherSuites,omitempty"`
	// TLS min version.
	TLSMinVersion string `yaml:"tlsMinVersion,omitempty"`
}
