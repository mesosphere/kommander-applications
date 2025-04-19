#!/usr/bin/env bash

set -euox pipefail

CI=${CI:-"true"}

ensure_prerequisites() { (
  # wait for hack/kind/gha/create-loop-devs_daemonset.yaml to finish running
  # currently, the e2e tests run the loopback creator daemonset if the CI env var is "true"
  echo "is this running in CI?: ${CI}"
  if [ "${CI}" == "true" ]; then
    timeout 5m bash -c "while ! test -e /dev/loop100; do echo 'waiting for loopback devices to be created'; sleep 5; done"
  fi

  if [ -x "$(command -v lvm)" ]; then
    echo "lvm is already installed"
    exit 0
  fi
  apt-get update
  apt-get install -y lvm2
); }

delete_lvm_based_on_hostname() { (
  readonly LOOPBACK_DEVICE=$1
  readonly VG_NAME=$2
  readonly LV_NAME=$3

  echo "Print hostname"
  cat /etc/hostname || true

  echo "Detach loop device"
  losetup -d "$LOOPBACK_DEVICE" || true

  echo "Deactivate an lvm"
  lvchange -an "/dev/$VG_NAME/$LV_NAME" || true

  echo "Remove an lvm"
  lvremove -f "/dev/$VG_NAME/$LV_NAME" || true

  echo "Deactivate volume group"
  vgchange -an "/dev/$VG_NAME" || true

  echo "Remove volume group"
  vgremove -f "/dev/$VG_NAME" || true

  echo "Remove LVM labels from physical volume"
  pvremove -f "$LOOPBACK_DEVICE" || true

  echo "Detach loop device"
  losetup -d "$LOOPBACK_DEVICE" || true

  BACKING_FILE=$(losetup -nO BACK-FILE "${LOOPBACK_DEVICE}")
  echo "Remove file backing the loopback storage : $BACKING_FILE"
  rm "$BACKING_FILE" || true

  echo "Try to remove the loopback device as a precaution"
  rm -rf "$LOOPBACK_DEVICE" || true

  echo "Remove any leftover ceph data on the node"
  rm -rf /var/lib/rook || true
); }

create_lvm_based_on_hostname() { (
  # Create a file image to back the loopback device
  # - Ceph is configured to consume one PV of size 1G
  echo "Create a file for loopback device"
  mkdir -p /hack

  while true; do
    FILE_ID=$(shuf -i 101-255 -n 1)
    if ! [ -f "/hack/file-vol${FILE_ID}" ]; then
      dd if=/dev/zero of="/hack/file-vol${FILE_ID}" bs=1M count=2048
      break
    fi
    echo "file already exists, retrying with a different ID"
  done

  echo "Print hostname"
  cat /etc/hostname || true

  echo "Set up the loopback device with file"
  while ! LOOPBACK_DEVICE=$(losetup -f --show --nooverlap "/hack/file-vol${FILE_ID}"); do
    echo "losetup failed, trying again"
    sleep 5
  done

  # get id from /dev/loop<id>
  LOOPBACK_ID="$(echo "${LOOPBACK_DEVICE}" | cut -d'p' -f2)"

  echo "${LOOPBACK_ID}" >>"${IDS_FILE}"

  VG_NAME="cephvg${LOOPBACK_ID}"
  LV_NAME="cephlv${LOOPBACK_ID}"

  # Wipe to remove any LVM metadata that may be present
  dd if=/dev/zero of="$LOOPBACK_DEVICE" bs=1M count=1024 oflag=direct,dsync

  echo "Initialize a physical volume for LVM backed by loopback device"
  pvcreate -ff --yes "${LOOPBACK_DEVICE}"

  echo "Create a volume group (uniqueness is guaranteed by hostname)"
  vgcreate "${VG_NAME}" "${LOOPBACK_DEVICE}"

  echo "Create a logical volume (uniqueness is guaranteed by hostname)"
  lvcreate --zero n --size 1G --name "${LV_NAME}" "${VG_NAME}"

  echo "Activate volume group"
  vgchange -a y "${VG_NAME}"

  echo "Create the special files for volume group"
  vgmknodes --refresh "${VG_NAME}"

  # rook discovery executes lsblk and relies on its output (it does not validate actual disk state etc..,)
  echo "Ensure rook discovery can see the lvm entries via lsblk output"
  lsblk "/dev/${VG_NAME}/${LV_NAME}" --bytes --nodeps --pairs --paths --output SIZE,ROTA,RO,TYPE,PKNAME,NAME,KNAME,MOUNTPOINT,FSTYPE
); }

cleanup() { (
  echo "performing cleanup of loopbackdeviceids"
  if [ -e "${IDS_FILE}" ]; then
    ids=$(cat "${IDS_FILE}")
    for id in $ids; do
      delete_lvm_based_on_hostname "/dev/loop${id}" "cephvg${id}" "cephlv${id}" "${id}" || true
    done
  fi
); }

readonly IDS_FILE="/hack/ceph/loopbackdeviceids"

######################### Delegates to subcommands or runs main, as appropriate
if [[ "${1-}" ]] && declare -F | cut -d' ' -f3 | grep -F -qx -- "${1-}"; then
  "$@"
else
  ensure_prerequisites
  rm -f "${IDS_FILE}" || true
  mkdir -p /hack/ceph/
  create_lvm_based_on_hostname
fi
