# AMD GPU Operator

This application deploys the **AMD GPU Operator** on Kubernetes via Flux (HelmRepository + HelmRelease), using the official ROCm Helm chart.

## Overview

The AMD GPU Operator automates the management of AMD software components needed to provision GPU nodes:

- **Node Feature Discovery (NFD)** – Detects AMD GPU hardware and labels nodes (e.g. `feature.node.kubernetes.io/amd-gpu=true`).
- **Kernel Module Management (KMM)** – Optional out-of-tree AMD GPU driver installation and lifecycle.
- **Device plugin** – Exposes `amd.com/gpu` as a schedulable resource.
- **Device Config Manager (DCM)** – GPU partitioning and configuration.
- **Metrics** – Optional device metrics exporter integration.

You can use **inbox or pre-installed drivers** (`spec.driver.enable: false` in DeviceConfig) or let the operator install **out-of-tree drivers** via KMM.

## Documentation

| Resource | Link |
|----------|------|
| **Installation (Helm)** | [Kubernetes (Helm) — AMD GPU Operator](https://instinct.docs.amd.com/projects/gpu-operator/en/latest/installation/kubernetes-helm.html) |
| **Release notes** | [Release Notes](https://instinct.docs.amd.com/projects/gpu-operator/en/latest/release_notes.html) |
| **Full config reference** | [Full Reference Config](https://instinct.docs.amd.com/projects/gpu-operator/en/latest/full_reference_config.html) |
| **DeviceConfig / CRDs** | [Custom Resource Installation](https://instinct.docs.amd.com/projects/gpu-operator/en/latest/installation/kubernetes-helm.html#install-custom-resource) |
| **Troubleshooting** | [Troubleshooting](https://instinct.docs.amd.com/projects/gpu-operator/en/latest/troubleshooting.html) |
| **Uninstall** | [Uninstallation](https://instinct.docs.amd.com/projects/gpu-operator/en/latest/uninstallation/uninstallation.html) |
| **GitHub** | [ROCm/gpu-operator](https://github.com/ROCm/gpu-operator) |

## Prerequisites

- Kubernetes cluster **v1.29.0 or later**
- Helm **v3.2.0 or later** (for catalog/Flux tooling)
- **cert-manager** installed (required for TLS; the operator uses it for webhooks)
- Worker nodes with **AMD GPUs**
- CNI and system pods healthy

## How This App Deploys It

- **Helm repository:** `https://rocm.github.io/gpu-operator`
- **Chart:** `gpu-operator-charts`
- **Version:** `v1.4.1`
- **Release name:** `amd-gpu-operator`
- **Target namespace:** Workspace release namespace (`releaseNamespace`)

Default values (ConfigMap) can enable the default DeviceConfig (`crds.defaultCR.install: true`). After install, create or edit a `DeviceConfig` in the operator namespace to select nodes (e.g. `selector: feature.node.kubernetes.io/amd-gpu: "true"`) and choose driver strategy (inbox vs out-of-tree).

## Configuration

- Override defaults via the app’s ConfigMap or your platform’s override mechanism.
- Key chart options: `node-feature-discovery.enabled`, `kmm.enabled`, `crds.defaultCR.install`, and sub-chart values (see [Helm chart customization](https://instinct.docs.amd.com/projects/gpu-operator/en/latest/installation/kubernetes-helm.html#helm-chart-customization-parameters)).
- For **inbox/pre-installed drivers**, set `spec.driver.enable: false` in your DeviceConfig.
- For **out-of-tree drivers**, set `spec.driver.enable: true` and `spec.driver.blacklist: true` (and reboot nodes as per AMD docs).

## Installing With AMD Network Operator

If you install both AMD GPU Operator and AMD Network Operator on the same cluster, follow AMD’s combined guide to avoid duplicate NFD/KMM and version conflicts: [Installation of GPU Operator and Network Operator together](https://instinct.docs.amd.com/projects/network-operator/en/main/installation/kubernetes-helm-operators.html).
