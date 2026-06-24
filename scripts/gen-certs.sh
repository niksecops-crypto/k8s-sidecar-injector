#!/bin/bash

# This script generates TLS certificates for the webhook server.
# It assumes you have 'openssl' and 'kubectl' installed.

SERVICE="sidecar-injector-service"
NAMESPACE="sidecar-injector"
SECRET="sidecar-injector-certs"

# Create a temporary directory for certs
TEMP_DIR=$(mktemp -d)
echo "Generating certs in $TEMP_DIR..."

# Generate CA key and cert
openssl genrsa -out $TEMP_DIR/ca.key 2048
openssl req -x509 -new -nodes -key $TEMP_DIR/ca.key -subj "/CN=$SERVICE.$NAMESPACE.svc" -days 3650 -out $TEMP_DIR/ca.crt

# Generate server key and CSR
openssl genrsa -out $TEMP_DIR/tls.key 2048
openssl req -new -key $TEMP_DIR/tls.key -subj "/CN=$SERVICE.$NAMESPACE.svc" -out $TEMP_DIR/tls.csr -config <(
cat <<EOF
[req]
req_extensions = v3_req
distinguished_name = dn
[dn]
[v3_req]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names
[alt_names]
DNS.1 = $SERVICE
DNS.2 = $SERVICE.$NAMESPACE
DNS.3 = $SERVICE.$NAMESPACE.svc
EOF
)

# Sign the server CSR with the CA
openssl x509 -req -in $TEMP_DIR/tls.csr -CA $TEMP_DIR/ca.crt -CAkey $TEMP_DIR/ca.key -CAcreateserial -out $TEMP_DIR/tls.crt -days 365 -extensions v3_req -extfile <(
cat <<EOF
[v3_req]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names
[alt_names]
DNS.1 = $SERVICE
DNS.2 = $SERVICE.$NAMESPACE
DNS.3 = $SERVICE.$NAMESPACE.svc
EOF
)

# Create the namespace if it doesn't exist
kubectl create namespace $NAMESPACE || true

# Create the secret in Kubernetes
kubectl create secret tls $SECRET --key=$TEMP_DIR/tls.key --cert=$TEMP_DIR/tls.crt -n $NAMESPACE --dry-run=client -o yaml | kubectl apply -f -

# Output the CA bundle (base64) for the webhook configuration
CA_BUNDLE=$(cat $TEMP_DIR/ca.crt | base64 | tr -d '\n')
echo "CA_BUNDLE for webhook-config.yaml:"
echo $CA_BUNDLE

# Cleanup
rm -rf $TEMP_DIR
echo "Done."
