#!/usr/bin/env bash
set -euo pipefail

NAMESPACE=${1:-kuberhealthy}
SERVICE=${2:-kuberhealthy}
SECRET_NAME=${3:-kuberhealthy-webhook-tls}
WEBHOOK_NAME=${4:-kuberhealthy-legacy-conversion}

TMPDIR=$(mktemp -d)
trap 'rm -rf "${TMPDIR}"' EXIT

CN="${SERVICE}.${NAMESPACE}.svc"
CRT="${TMPDIR}/tls.crt"
KEY="${TMPDIR}/tls.key"
CA_KEY="${TMPDIR}/ca.key"
CA_CERT="${TMPDIR}/ca.crt"
CSR="${TMPDIR}/server.csr"

cat <<EOF2 > "${TMPDIR}/csr.conf"
[req]
req_extensions = v3_req
distinguished_name = req_distinguished_name
prompt = no

[req_distinguished_name]
CN = ${CN}

[v3_req]
keyUsage = critical, digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names

[alt_names]
DNS.1 = ${CN}
DNS.2 = ${SERVICE}
DNS.3 = ${SERVICE}.${NAMESPACE}
DNS.4 = ${SERVICE}.${NAMESPACE}.svc.cluster.local
EOF2

cat <<EOF3 > "${TMPDIR}/ca.conf"
[req]
req_extensions = v3_ca
distinguished_name = req_distinguished_name
prompt = no

[req_distinguished_name]
CN = ${SERVICE}.${NAMESPACE}.svc.cluster.local

[v3_ca]
subjectKeyIdentifier = hash
authorityKeyIdentifier = keyid:always,issuer
basicConstraints = critical, CA:true
keyUsage = critical, digitalSignature, cRLSign, keyCertSign
EOF3

openssl genrsa -out "${CA_KEY}" 4096 >/dev/null 2>&1
openssl req -x509 -new -nodes -key "${CA_KEY}" -sha256 -days 3650 \
  -out "${CA_CERT}" -config "${TMPDIR}/ca.conf" >/dev/null 2>&1

openssl genrsa -out "${KEY}" 2048 >/dev/null 2>&1
openssl req -new -key "${KEY}" -out "${CSR}" -config "${TMPDIR}/csr.conf" >/dev/null 2>&1
openssl x509 -req -in "${CSR}" -CA "${CA_CERT}" -CAkey "${CA_KEY}" -CAcreateserial \
  -out "${CRT}" -days 3650 -sha256 -extensions v3_req -extfile "${TMPDIR}/csr.conf" >/dev/null 2>&1

kubectl create secret tls "${SECRET_NAME}" \
  --namespace "${NAMESPACE}" \
  --cert="${CRT}" \
  --key="${KEY}" \
  --dry-run=client \
  -o yaml | kubectl apply -f -

BASE64_CA=$(base64 -w0 < "${CA_CERT}")

kubectl patch mutatingwebhookconfiguration "${WEBHOOK_NAME}" \
  --type='json' \
  -p="[{\"op\":\"replace\",\"path\":\"/webhooks/0/clientConfig/caBundle\",\"value\":\"${BASE64_CA}\"}]"

echo "Updated webhook certificate and CA bundle"
