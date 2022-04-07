#!/usr/bin/env bash
set -euo pipefail
set -x

cp /opt/scripts/update-certs.timer /etc/systemd/system/update-certs.timer
cp /opt/scripts/update-certs.service /etc/systemd/system/update-certs.service
cp /opt/scripts/update-certs.path /etc/systemd/system/update-certs.path

systemctl enable --now update-certs.path
systemctl start update-certs.path
systemctl enable --now update-certs.timer

timerStatus=$(systemctl is-enabled update-certs.timer)
pathStatus=$(systemctl is-enabled update-certs.path)

if [[ $timerStatus == "enabled" && $pathStatus == "enabled" ]]; then
        exit 0
else
        echo "Failed to enable timer or path, cleaning up" >&2
        rm /etc/systemd/system/update-certs.service /etc/systemd/system/update-certs.timer /etc/systemd/system/update-certs.path
        exit 1
fi
