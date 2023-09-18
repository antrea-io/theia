# Using Ingress to connect Flow Aggregator to ClickHouse Server

## Table of Contents

<!-- toc -->
- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Step 1: Install and Configure Ingress Controller](#step-1-install-and-configure-ingress-controller)
- [Step 2: Expose ClickHouse Service via Ingress](#step-2-expose-clickhouse-service-via-ingress)
- [Step 3: Create TLS Secret (Optional)](#step-3-create-tls-secret-optional)
  - [Using kubectl](#using-kubectl)
  - [Using cert-manager](#using-cert-manager)
- [Step 4: Configure Antrea Flow Aggregator](#step-4-configure-antrea-flow-aggregator)
<!-- /toc -->

## Overview

In this guide, we'll walk you through the process of setting up Kubernetes Ingress
with ClickHouse as the backend service and how to connect your Flow Aggregator to
a ClickHouse server. Kubernetes Ingress is a powerful way to manage external
access to services within your Kubernetes cluster, and ClickHouse is a popular
open-source columnar database management system. By using Ingress, you can easily
expose ClickHouse to external clients and manage routing and load balancing
efficiently.

## Prerequisites

Before proceeding with the setup, ensure that you have the following prerequisites:

1. A Kubernetes cluster up and running.
2. kubectl command-line tool installed and configured to access your Kubernetes cluster.
3. ClickHouse server deployed within your Kubernetes cluster.

## Step 1: Install and Configure Ingress Controller

The first step is to install an Ingress controller in your Kubernetes cluster.
There are various Ingress controllers available, such as Nginx Ingress Controller,
Traefik, etc. Choose one that suits your requirements and install it following
the documentation provided by the controller's maintainers.

Here we use Nginx Ingress Controller as an example:

```bash
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.8.1/deploy/static/provider/cloud/deploy.yaml
```

## Step 2: Expose ClickHouse Service via Ingress

Assuming you have already deployed ClickHouse as a Kubernetes service, you
need to expose it using an Ingress resource. Create an Ingress manifest file
(e.g., clickhouse-ingress.yaml) with the following content:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: clickhouse-ingress
  namespace: flow-visibility   # Replace with your desired namespace
spec:
  ingressClassName: nginx   # Replace with your desired class name or omit it for using default ingress class
  rules:
    - host: clickhouse.example.com   # Replace with your desired domain/subdomain
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: clickhouse-clickhouse    # Replace with your ClickHouse service name
                port:
                  number: 8123             # Replace with your ClickHouse service port
  tls: # optional
    - hosts:
        - clickhouse.example.com       # Replace with your desired domain/subdomain
      secretName: clickhouse-tls-secret   # Replace with the name of your TLS secret
```

Save the file and deploy the Ingress resource using kubectl:

```bash
kubectl apply -f clickhouse-ingress.yaml
```

Ensure that the Ingress resource is created successfully:

```bash
kubectl get ingress clickhouse-ingress
```

## Step 3: Create TLS Secret (Optional)

If you don't have a TLS secret already, you need to create one. A TLS secret
contains the SSL/TLS certificate and a private key used for secure HTTPS
connections. You can obtain a TLS certificate from a trusted certificate
authority or generate a self-signed certificate for testing purposes.

### Using kubectl

To create a TLS secret, use the following command:

```bash
kubectl create secret tls clickhouse-tls-secret --cert=path/to/cert.crt --key=path/to/key.key
```

Replace `clickhouse-tls-secret` with the desired name for the secret, and
`path/to/cert.crt` and `path/to/key.key` with the paths to your TLS certificate
and private key files.

### Using cert-manager

If you set up [cert-manager](https://cert-manager.io/docs/) to manage your
certificates, it can be used to issue and renew the certificate required by
Ingress.

To get started, follow the [cert-manager installation documentation](
https://cert-manager.io/docs/installation/kubernetes/) to deploy cert-manager
and configure `Issuer` or `ClusterIssuer` resources.

Proceed to the [Securing Ingress Resources](https://cert-manager.io/docs/usage/ingress/)
documentation to learn about adding annotations to your Ingress. If you're
utilizing a cluster-issuer, your Ingress configuration would appear as follows:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: clickhouse-ingress
  namespace: flow-visibility   # Replace with your desired namespace
  annotations:
    # add an annotation indicating the issuer to use. Replace the name with the real Issuer you configured.
    cert-manager.io/cluster-issuer: nameOfClusterIssuer
spec:
  ingressClassName: nginx   # Replace with your desired class name or omit it for using default ingress class
  rules:
    - host: clickhouse.example.com   # Replace with your desired domain/subdomain
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: clickhouse-clickhouse    # Replace with your ClickHouse service name
                port:
                  number: 8123             # Replace with your ClickHouse service port
  tls: # optional
    - hosts:
        - clickhouse.example.com       # Replace with your desired domain/subdomain
      secretName: clickhouse-tls-secret   # Replace with the name of your TLS secret
```

Once the `Ingress` is created, you should see the `clickhouse-tls-secret`
Secret created in the `flow-visibility` Namespace.

## Step 4: Configure Antrea Flow Aggregator

To access ClickHouse using the hostname defined in the Ingress
(e.g., clickhouse.example.com), you must configure `databaseURL` and
`hostAliases` to your Flow Aggregator yaml file.

Change `databseURL` and `hostAliases` in `flow-aggregator.conf`:

```yaml
flow-aggregator.conf: |
  # -- HostAliases to be injected into the Pod's hosts file.
  # For example: `[{"ip": "8.8.8.8", "hostnames": ["clickhouse.example.com"]}]`
  hostAliases: [{"ip": "8.8.8.8", "hostnames": ["clickhouse.example.com"]}]
  ...
  # clickHouse contains ClickHouse related configuration options.
  clickHouse:
    # Enable is the switch to enable exporting flow records to ClickHouse.
    enable: true

    # Database is the name of database where Antrea "flows" table is created.
    database: "default"

    # DatabaseURL is the url to the database. Provide the database URL as a string with format
    # <Protocol>://<ClickHouse server FQDN or IP>:<ClickHouse port>. The protocol has to be
    # one of the following: "tcp", "tls", "http", "https". When "tls" or "https" is used, tls
    # will be enabled.
    databaseURL: "http://clickhouse.example.com"
```

Change `8.8.8.8` to the cluster's external IP where you created the Ingress.
Change `clickhouse.example.com` to your hostname.

If you created TLS Secret in Step 3, please follow the document
[configuration-secure-connection-to-the-clickhouse-database](https://github.com/antrea-io/antrea/blob/main/docs/network-flow-visibility.md#configuring-secure-connections-to-the-clickhouse-database)
to create `clickhouse-ca` Secret and change the `databaseURL` to
`"https://clickhouse.example.com"`.

You've now successfully set up Kubernetes Ingress with ClickHouse as the backend
service.
