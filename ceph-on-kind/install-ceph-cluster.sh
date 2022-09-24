#!/usr/bin/env bash


helm install --dry-run --debug rook-ceph-cluster rook-release/rook-ceph-cluster -f rook-ceph-cluster-values-raw.yaml
