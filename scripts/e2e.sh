#!/usr/bin/env bash
set -euo pipefail
set -x

clean_up() {
  kind delete cluster
}

trap clean_up EXIT ERR

poll_container() {
  n=0; until ((n >= $2)); do $1 && break; n=$((n + 1)); sleep 5; done; ((n < $2))
}

check_if_first_cert_appeared_at_source() {
  docker exec -i "${containerName}" bash <<EOF
  if [ -z \$(ls -A "$certSource") ]; then
     echo "Cert files from secret didn't appear in /opt/certs" >&2
     exit 1
  fi
EOF
}

check_if_first_cert_copied_to_dest() {
  docker exec -i "${containerName}" bash <<EOF
  if [[ ! -f ""$certDestination"/"$firstCertName"" ]]; then
     echo "expected "$firstCertName" in "$certDestination" but not found" >&2
     exit 1
  fi
EOF
}

check_if_first_cert_added_to_trusted_ca() {
  docker exec -i "${containerName}" bash <<EOF
  if [[ ! -f "/etc/ssl/certs/"$firstCertNameNoExt".pem" ]]; then
     echo "expected "$firstCertNameNoExt" in /etc/ssl/certs but not found" >&2
     exit 1
  fi
EOF
}

check_if_first_cert_removed_from_dest() {
  docker exec -i "${containerName}" bash <<EOF
  if [[ -f ""$certDestination"/"$firstCertName"" ]]; then
       echo "old file ($firstCertName) not removed from $certDestination" >&2
       exit 1
  fi
EOF
}

check_if_first_cert_removed_from_source() {
  docker exec -i "${containerName}" bash <<EOF
  if [[ -f ""$certSource"/"$firstCertName"" ]]; then
      echo "old file ($firstCertName) not removed from $certSource" >&2
      exit 1
  fi
EOF
}

check_if_first_cert_removed_from_trusted_ca() {
  docker exec -i "${containerName}" bash <<EOF
  for file in "/etc/ssl/certs"/*; do
    name=\${file##*/}
     if [[ \$name == "$firstCertName" ]]; then
         echo "old file ("$firstCertName") not removed from /etc/ssl/certs" >&2
         exit 1
     fi
  done
EOF
}

check_if_new_certs_appeared_at_source () {
  docker exec -i "${containerName}" bash <<EOF
  srcFileCount=\$(find $certSource -type f -name "*.crt" | wc -l)
  if [[ \$srcFileCount -ne 2 ]]; then
    echo "Incorrect amount of certs found in $certSource, expected 2, found \$srcFileCount" >&2
    exit 1
  fi
EOF
}

check_if_new_certs_appeared_at_dest () {
  docker exec -i "${containerName}" bash <<EOF
  destFileCount=\$(find $certDestination -type f -name "*.crt" | wc -l)
  if [[ \$destFileCount -ne 2 ]]; then
    echo "Incorrect amount of certs found in $certDestination, expected 2, found \$destFileCount" >&2
    exit 1
  fi
EOF
}

check_if_new_certs_added_to_trusted_ca() {
  docker exec -i "${containerName}" bash <<EOF
  for file in "$certDestination"/*; do
    name=\${file##*/}
    nameWithoutExt=\${name%.*}
     if [[ ! -f "/etc/ssl/certs/\$nameWithoutExt.pem" ]]; then
         echo "\$nameWithoutExt expected in /etc/ssl/certs but not found" >&2
         exit 1
     fi
  done
EOF
}

check_if_source_dir_empty() {
  docker exec -i "${containerName}" bash <<EOF
  if [[ ! -z \$(ls -A "$certSource") ]]; then
     echo "Cert files not removed from $certSource after secret was removed" >&2
     exit 1
  fi
EOF
}

check_if_dest_dir_empty() {
  docker exec -i "${containerName}" bash <<EOF
  if [[ ! -z \$(ls -A "$certDestination") ]]; then
     echo "Cert files not removed from $certDestination after secret was removed" >&2
     exit 1
  fi
EOF
}

check_if_no_tagged_files_in_trusted_ca() {
  docker exec -i "${containerName}" bash <<EOF
  if [[ $(ls /etc/ssl/certs | grep -cE '^[0-9]{14}') != 0 ]]; then
     echo "Cert files not removed from /etc/ssl/certs (Trusted CA) after secret was removed" >&2
     exit 1
  fi
EOF
}

# 1. Create kind cluster
kind create cluster
kubectl cluster-info --context kind-kind
poll_container "kubectl -n default get serviceaccount default -o name" 60
containerName=$(docker ps -q | head -n 1)

# 2. Build docker image
docker build .. -t aks-node-ca-watcher:latest

# 3. Load image to kind cluster
kind load docker-image aks-node-ca-watcher

# 4. Setup systemd unit and timer
docker cp . "${containerName}":/opt/scripts/

# 5. Run daemonset with aks-node-ca-watcher and run services
kubectl apply -f ../manifests/trustedCADS.yaml

# 6. Start systemd timer/service
docker exec "${containerName}" chmod -R +x /opt/scripts

docker exec "${containerName}" /opt/scripts/setup_service.sh
certSource=/opt/certs
certDestination=/usr/local/share/ca-certificates/certs

# 7. Check that first cert file appeared in /opt/cert on node
poll_container check_if_first_cert_appeared_at_source 60

# Get name of the first cert
firstCertPath=$(docker exec -i "${containerName}" bash <<EOF
if [ -z \$(ls -A "$certSource") ]; then
   echo "Cert files from secret didn't appear in /opt/certs" >&2
   exit 1
else
  currentCerts=("$certSource"/*)
  echo \${currentCerts[0]}
fi
EOF
)
firstCertName=${firstCertPath##*/}
firstCertNameNoExt=${firstCertName%.*}

#check if first cert is copied to /usr/local/share/ca-certificates/certs on node
poll_container check_if_first_cert_copied_to_dest 60

# check if first cert file is present at /etc/ssl/certs
poll_container check_if_first_cert_added_to_trusted_ca 60

# 8. Change secret with certs - remove old one, add two new ones
kubectl delete secret trustedcasecret
kubectl apply -f ../manifests/updatedCerts.yaml

# 9. Assert that old file is removed and new ones are added correctly
# check if new file appeared in /opt/certs
poll_container check_if_new_certs_appeared_at_source 30

# check if old file is removed from /opt/certs after new ones arrive
poll_container check_if_first_cert_removed_from_source 30

# check if new files are in /usr/local/share/ca-certificates/certs
poll_container check_if_new_certs_appeared_at_dest 30

#check if first cert is removed from /usr/local/share/ca-certificates/certs after new ones arrive
check_if_first_cert_removed_from_dest

# check if new file is visible in /etc/ssl/certs and old file is no longer there
poll_container check_if_new_certs_added_to_trusted_ca 60

check_if_first_cert_removed_from_trusted_ca

# 10. Remove secret and check if all files are removed from node
kubectl delete secret trustedcasecret
kubectl apply -f ../manifests/emptySecret.yaml

poll_container check_if_source_dir_empty 30

poll_container check_if_dest_dir_empty 30

poll_container check_if_no_tagged_files_in_trusted_ca 30

exit 0