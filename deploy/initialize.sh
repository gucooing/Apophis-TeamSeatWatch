#!/bin/sh
set -eu
umask 077

database=/state/database
control=/state/control
gateway=/state/gateway
mkdir -p "$database" "$control/authority" "$control/tls" "$gateway"

# Reuse all existing material on restarts and upgrades. Publish each new file
# atomically so an interrupted initializer cannot leave a truncated secret.
if [ ! -f "$database/password" ]; then
  openssl rand -hex 32 > "$database/password.tmp"
  mv "$database/password.tmp" "$database/password"
fi
# POSTGRES_PASSWORD_FILE must also be readable after PostgreSQL drops root.
chmod 444 "$database/password"
if [ ! -f "$control/database-url" ]; then
  printf 'postgres://teamseatwatch:%s@postgres:5432/teamseatwatch?sslmode=disable\n' "$(cat "$database/password")" > "$control/database-url.tmp"
  mv "$control/database-url.tmp" "$control/database-url"
fi
if [ ! -f "$control/keyring.json" ]; then
  key=$(openssl rand -base64 32 | tr '/+' '_-' | tr -d '=\n')
  printf '{"current_version":1,"keys":{"1":"%s"}}\n' "$key" > "$control/keyring.json.tmp"
  mv "$control/keyring.json.tmp" "$control/keyring.json"
fi

# These certificates authenticate the private control/gateway connection.
# Browser-facing listeners are HTTP and require no generated web certificate.
if [ ! -f "$control/authority/ca.key" ]; then
  if [ -f "$control/authority/ca.crt" ]; then
    printf '%s\n' 'Private CA key is missing; restore the matching secrets volume.' >&2
    exit 1
  fi
  openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:3072 -out "$control/authority/ca.key.tmp"
  mv "$control/authority/ca.key.tmp" "$control/authority/ca.key"
fi
if [ ! -f "$control/authority/ca.crt" ]; then
  openssl req -x509 -new -sha256 -days 3650 -key "$control/authority/ca.key" \
    -out "$control/authority/ca.crt.tmp" -subj '/CN=Private Service CA' \
    -addext 'basicConstraints=critical,CA:TRUE' -addext 'keyUsage=critical,keyCertSign,cRLSign'
  mv "$control/authority/ca.crt.tmp" "$control/authority/ca.crt"
fi

issue_certificate() {
  folder=$1
  name=$2
  common_name=$3
  usage=$4
  san=$5
  if [ ! -f "$folder/$name.key" ]; then
    openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:3072 -out "$folder/$name.key.tmp"
    mv "$folder/$name.key.tmp" "$folder/$name.key"
  fi
  if [ ! -f "$folder/$name.crt" ]; then
    printf 'basicConstraints=critical,CA:FALSE\nkeyUsage=critical,digitalSignature,keyEncipherment\nextendedKeyUsage=%s\n' "$usage" > "$folder/extensions.cnf"
    if [ -n "$san" ]; then printf 'subjectAltName=%s\n' "$san" >> "$folder/extensions.cnf"; fi
    openssl req -new -key "$folder/$name.key" -out "$folder/$name.csr" -subj "/CN=$common_name"
    openssl x509 -req -sha256 -days 365 -in "$folder/$name.csr" \
      -CA "$control/authority/ca.crt" -CAkey "$control/authority/ca.key" \
      -set_serial "0x$(openssl rand -hex 16)" -extfile "$folder/extensions.cnf" -out "$folder/$name.crt.tmp"
    mv "$folder/$name.crt.tmp" "$folder/$name.crt"
    rm -f "$folder/$name.csr" "$folder/extensions.cnf"
  fi
}

issue_certificate "$control/tls" server control serverAuth 'DNS:control'
issue_certificate "$gateway" client gateway clientAuth ''
cp "$control/authority/ca.crt" "$gateway/ca.crt.tmp"
mv "$gateway/ca.crt.tmp" "$gateway/ca.crt"
printf '%s\n' 'Deployment material is ready; existing keys and passwords were reused.'
