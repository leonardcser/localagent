#!/bin/sh
if [ -f /usr/local/share/ca-certificates/custom-ca.crt ]; then
    cp /etc/ssl/certs/ca-certificates.crt /tmp/ca-bundle.crt
    cat /usr/local/share/ca-certificates/custom-ca.crt >> /tmp/ca-bundle.crt
    export SSL_CERT_FILE=/tmp/ca-bundle.crt
fi
exec localagent "$@"
