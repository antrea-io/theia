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

import (
	"context"
	"fmt"
	"net"
	"os"
	"path"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/server/dynamiccertificates"
	"k8s.io/apiserver/pkg/server/options"
	"k8s.io/client-go/kubernetes"
	certutil "k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/keyutil"
	"k8s.io/klog/v2"

	"antrea.io/theia/pkg/util/env"
)

const (
	// The names of the files that should contain the CA certificate and the TLS key pair.
	CACertFile  = "ca.crt"
	TLSCertFile = "tls.crt"
	TLSKeyFile  = "tls.key"
)

// GetTheiaServerNames returns the DNS names that the TLS certificate will be signed with.
func GetTheiaServerNames(serviceName string) []string {
	namespace := env.GetTheiaNamespace()
	theiaServerName := serviceName + "." + namespace + ".svc"
	// TODO: Add the whole FQDN "theia-manager.<Namespace>.svc.<Cluster Domain>" as an
	// alternate DNS name when other clients need to access it directly with that name.
	return []string{theiaServerName}
}

func ApplyServerCert(selfSignedCert bool,
	client kubernetes.Interface,
	secureServing *options.SecureServingOptionsWithLoopback,
	caConfig *CAConfig) (*CACertController, error) {
	var err error
	var caContentProvider dynamiccertificates.CAContentProvider
	if selfSignedCert {
		caContentProvider, err = generateSelfSignedCertificate(secureServing, caConfig)
		if err != nil {
			return nil, fmt.Errorf("error creating self-signed CA certificate: %v", err)
		}
	} else {
		caCertPath := path.Join(caConfig.CertDir, CACertFile)
		tlsCertPath := path.Join(caConfig.CertDir, TLSCertFile)
		tlsKeyPath := path.Join(caConfig.CertDir, TLSKeyFile)
		// The secret may be created after the Pod is created, for example, when cert-manager is used the secret
		// is created asynchronously. It waits for a while before it's considered to be failed.
		if err = wait.PollImmediate(2*time.Second, caConfig.CertReadyTimeout, func() (bool, error) {
			for _, path := range []string{caCertPath, tlsCertPath, tlsKeyPath} {
				f, err := os.Open(path)
				if err != nil {
					klog.Warningf("Couldn't read %s when applying server certificate, retrying", path)
					return false, nil
				}
				f.Close()
			}
			return true, nil
		}); err != nil {
			return nil, fmt.Errorf("error reading TLS certificate and/or key. Please make sure the TLS CA (%s), cert (%s), and key (%s) files are present in \"%s\", when selfSignedCert is set to false", CACertFile, TLSCertFile, TLSKeyFile, caConfig.CertDir)
		}
		// Since 1.17.0 (https://github.com/kubernetes/kubernetes/commit/3f5fbfbfac281f40c11de2f57d58cc332affc37b),
		// apiserver reloads certificate cert and key file from disk every minute, allowing serving tls config to be updated.
		secureServing.ServerCert.CertKey.CertFile = tlsCertPath
		secureServing.ServerCert.CertKey.KeyFile = tlsKeyPath

		caContentProvider, err = dynamiccertificates.NewDynamicCAContentFromFile("user-provided CA cert", caCertPath)
		if err != nil {
			return nil, fmt.Errorf("error reading user-provided CA certificate: %v", err)
		}
	}

	caCertController := newCACertController(caContentProvider, client, caConfig)

	if selfSignedCert {
		go rotateSelfSignedCertificates(caCertController, secureServing, caConfig.MaxRotateDuration)
	}

	return caCertController, nil
}

// generateSelfSignedCertificate generates a new self signed certificate.
func generateSelfSignedCertificate(secureServing *options.SecureServingOptionsWithLoopback, caConfig *CAConfig) (dynamiccertificates.CAContentProvider, error) {
	var err error
	var caContentProvider dynamiccertificates.CAContentProvider

	// Set the PairName and CertDirectory to generate the certificate files.
	secureServing.ServerCert.CertDirectory = caConfig.SelfSignedCertDir
	secureServing.ServerCert.PairName = caConfig.PairName

	if err := secureServing.MaybeDefaultWithSelfSignedCerts(caConfig.ServiceName, GetTheiaServerNames(caConfig.ServiceName), []net.IP{net.ParseIP("127.0.0.1"), net.IPv6loopback}); err != nil {
		return nil, fmt.Errorf("error creating self-signed certificates: %v", err)
	}

	caContentProvider, err = dynamiccertificates.NewDynamicCAContentFromFile("self-signed cert", secureServing.ServerCert.CertKey.CertFile)
	if err != nil {
		return nil, fmt.Errorf("error reading self-signed CA certificate: %v", err)
	}

	return caContentProvider, nil
}

// Used to determine which is sooner, the provided maxRotateDuration or the expiration date
// of the cert. Used to allow for unit testing with a far shorter rotation period.
// Also can be used to pass a user provided rotation window.
func nextRotationDuration(secureServing *options.SecureServingOptionsWithLoopback,
	maxRotateDuration time.Duration) (time.Duration, error) {

	x509Cert, err := certutil.CertsFromFile(secureServing.ServerCert.CertKey.CertFile)
	if err != nil {
		return time.Duration(0), fmt.Errorf("error parsing generated certificate: %v", err)
	}

	// Attempt to rotate the certificate at the half-way point of expiration.
	// Unless the halfway point is longer than maxRotateDuration
	duration := x509Cert[0].NotAfter.Sub(time.Now()) / 2

	waitDuration := duration
	if maxRotateDuration < waitDuration {
		waitDuration = maxRotateDuration
	}

	return waitDuration, nil
}

// rotateSelfSignedCertificates calculates the rotation duration for the current certificate.
// Then once the duration is complete, generates a new self-signed certificate and repeats the process.
func rotateSelfSignedCertificates(c *CACertController, secureServing *options.SecureServingOptionsWithLoopback,
	maxRotateDuration time.Duration) {
	for {
		rotationDuration, err := nextRotationDuration(secureServing, maxRotateDuration)
		if err != nil {
			klog.ErrorS(err, "Error when reading expiration date of cert")
			return
		}

		klog.InfoS("Certificate will be rotated at", "time", time.Now().Add(rotationDuration))

		time.Sleep(rotationDuration)

		klog.InfoS("Rotating self signed certificate")

		err = generateNewServingCertificate(secureServing, c.caConfig)
		if err != nil {
			klog.ErrorS(err, "Error when generating new cert")
			return
		}
		c.UpdateCertificate(context.TODO())
	}
}

func generateNewServingCertificate(secureServing *options.SecureServingOptionsWithLoopback, caConfig *CAConfig) error {
	cert, key, err := certutil.GenerateSelfSignedCertKeyWithFixtures(caConfig.ServiceName, []net.IP{net.ParseIP("127.0.0.1"), net.IPv6loopback}, GetTheiaServerNames(caConfig.ServiceName), secureServing.ServerCert.FixtureDirectory)
	if err != nil {
		return fmt.Errorf("unable to generate self signed cert: %v", err)
	}

	if err := certutil.WriteCert(secureServing.ServerCert.CertKey.CertFile, cert); err != nil {
		return err
	}
	if err := keyutil.WriteKey(secureServing.ServerCert.CertKey.KeyFile, key); err != nil {
		return err
	}
	klog.InfoS("Generated self-signed certificate", "cert", secureServing.ServerCert.CertKey.CertFile,
		"key", secureServing.ServerCert.CertKey.KeyFile)

	return nil
}
