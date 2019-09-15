#!/usr/bin/env bash

#openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -nodes -days 365 -subj '/CN=localhost'

case $(uname -s) in
Linux*) sslConfig=/etc/ssl/openssl.cnf ;;
Darwin*) sslConfig=/System/Library/OpenSSL/openssl.cnf ;;
esac
openssl req \
  -newkey rsa:2048 \
  -x509 \
  -nodes \
  -keyout server.key \
  -new \
  -out server.pem \
  -subj /CN=localhost \
  -reqexts SAN \
  -extensions SAN \
  -config <(cat $sslConfig \
    <(printf '[SAN]\nsubjectAltName=DNS:localhost')) \
  -sha256 \
  -days 3650
