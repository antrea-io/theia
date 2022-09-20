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

package certificate

import "time"

const (
	TheiaCAConfigMapName = "theia-ca"
	TheiaServiceName     = "theia-manager"
)

type CAConfig struct {
	// Name of the ConfigMap that will hold the CA certificate that signs the TLS
	// certificate of theia manager.
	CAConfigMapName string

	// CertDir is the directory that the TLS Secret should be mounted to. Declaring it as a variable for testing.
	CertDir string

	// SelfSignedCertDir is the dir self-signed certificates are created in.
	SelfSignedCertDir string

	// CertReadyTimeout is the timeout we will wait for the TLS Secret being ready. Declaring it as a variable for testing.
	CertReadyTimeout time.Duration

	// MaxRotateDuration is the max duration for rotating self-signed certificate generated.
	// In most cases we will rotate the certificate when we reach half the expiration time of the certificate (see nextRotationDuration).
	// MaxRotateDuration ensures that if a self-signed certificate has a really long expiration (N years), we still attempt to rotate it
	// within a reasonable time, in this case one year. maxRotateDuration is also used to force certificate rotation in unit tests.
	MaxRotateDuration time.Duration
	ServiceName       string
	PairName          string
}
