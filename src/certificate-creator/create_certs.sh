#!/bin/bash

# create TLS certificates 

# based on https://github.com/nodejs/help/issues/253

if test $# -ne 3
then
    echo "Wrong number of arguments"
    echo "./create_certs PATH FQDN PASSWORD"
    echo './create_certs . localhost passw0rd'
    exit 1
fi

ROOTPATH="$1"
FQDN=$2
PASSWORD=$3
RSABITS=4096

# make directories to work from
mkdir -p $ROOTPATH/certs/{server,client,ca,tmp}

PATH_CA=$ROOTPATH/certs/ca
PATH_SERVER=$ROOTPATH/certs/server
PATH_CLIENT=$ROOTPATH/certs/client
PATH_TMP=$ROOTPATH/certs/tmp

######
# CA #
######

openssl genrsa -des3 -passout pass:$PASSWORD -out $PATH_CA/ca.key $RSABITS 

# Create Authority Certificate
openssl req -new -x509 -days 36500 -key $PATH_CA/ca.key -out $PATH_CA/ca.crt -passin pass:$PASSWORD -subj "/C=FR/ST=./L=./O=ACME Signing Authority Inc/CN=CA_NAME"

##########
# SERVER #
##########

# Generate server key
openssl genrsa -out $PATH_SERVER/server.key $RSABITS

# Generate server cert
openssl req -newkey -x509 -key $PATH_SERVER/server.key -out $PATH_TMP/server.csr -passout pass:$PASSWORD -subj "/C=FR/ST=./L=./O=ACME Signing Authority Inc/CN=$FQDN" -config server.conf 

# Sign server cert with self-signed cert
openssl x509 -req -days 36500 -passin pass:$PASSWORD -in $PATH_TMP/server.csr -CA $PATH_CA/ca.crt -CAkey $PATH_CA/ca.key -set_serial 01 -out $PATH_SERVER/server.crt -extfile server.conf -extensions v3_req

openssl pkcs12 -export -in $PATH_CA/ca.crt -in $PATH_SERVER/server.crt -inkey $PATH_SERVER/server.key -out $PATH_SERVER/server.pfx 


##########
# CLIENT #
##########

openssl genrsa -out $PATH_CLIENT/client.key $RSABITS

openssl req -newkey -x509 -key $PATH_CLIENT/client.key -out $PATH_TMP/client.csr -passout pass:$PASSWORD -subj "/C=FR/ST=./L=./O=ACME Signing Authority Inc/CN=CLIENT_NAME" 

openssl x509 -req -days 36500 -passin pass:$PASSWORD -in $PATH_TMP/client.csr -CA $PATH_CA/ca.crt -CAkey $PATH_CA/ca.key -set_serial 01 -out $PATH_CLIENT/client.crt 

openssl pkcs12 -export -in $PATH_CLIENT/ca.crt -in $PATH_CLIENT/client.crt -inkey $PATH_CLIENT/client.key -out $PATH_CLIENT/client.pfx 

exit 0