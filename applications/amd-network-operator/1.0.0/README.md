# AMD Network Operator

This application deploys the **AMD Network Operator** on Kubernetes via Flux (HelmRepository + HelmRelease), using the official ROCm Helm chart.

## Overview

The AMD Network Operator simplifies deployment and management of **AMD AI NICs (AINICs)** in Kubernetes:

- **Node Feature Discovery (NFD)** – Detects AMD NIC hardware and labels nodes (e.g. `feature.node.kubernetes.io/amd-nic=true`).
- **Kernel Module Management (KMM)** – Driver and firmware lifecycle for AMD NICs.
- **Multus** – Optional multi-network support.
- **NIC device plugin / resources** – Exposes AMD NIC resources (e.g. `amd.com/nic`, `amd.com/vnic`) for workloads.
- **Metrics** – Prometheus-compatible metrics for NIC health and utilization.

It is designed to work alongside the **AMD GPU Operator** for high-performance networking in AI/ML and HPC clusters.

## Documentation

| Resource | Link |
|----------|------|
| **Installation (Helm)** | [Kubernetes (Helm) — Network Operator](https://instinct.docs.amd.com/projects/network-operator/en/main/installation/kubernetes-helm.html) |
| **NetworkConfig CRD** | [Custom Resource Installation / NetworkConfig](https://instinct.docs.amd.com/projects/network-operator/en/main/installation/networkconfig.html) |
| **Helm chart parameters** | [Helm Chart Customization Parameters](https://instinct.docs.amd.com/projects/network-operator/en/main/installation/kubernetes-helm.html#helm-chart-customization-parameters) (and `helm show values rocm-network/network-operator-charts`) |
| **Workload example** | [Test a Workload Deployment](https://instinct.docs.amd.com/projects/network-operator/en/main/installation/workload.html) |
| **Troubleshooting** | [Troubleshooting](https://instinct.docs.amd.com/projects/network-operator/en/main/troubleshooting.html) |
| **Uninstall** | [Uninstallation](https://instinct.docs.amd.com/projects/network-operator/en/main/uninstallation/uninstallation.html) |
| **GitHub** | [ROCm/network-operator](https://github.com/ROCm/network-operator) |

## Prerequisites

- Kubernetes cluster **v1.29.0 or later**
- Helm **v3.2.0 or later** (for catalog/Flux tooling)
- **cert-manager** installed (required for TLS)
- Worker nodes with **AMD NICs** (e.g. AMD Pensando Pollara AI NIC)
- CNI and system pods healthy

## How This App Deploys It

- **Helm repository:** `https://rocm.github.io/network-operator`
- **Chart:** `network-operator-charts`
- **Version:** `v1.0.0`
- **Release name:** `amd-network-operator`
- **Target namespace:** Workspace release namespace (`releaseNamespace`)

After the operator is installed, create a **NetworkConfig** custom resource in the operator namespace so the operator can manage NICs (driver install, device plugin, etc.). See the [NetworkConfig guide](https://instinct.docs.amd.com/projects/network-operator/en/main/installation/networkconfig.html) for examples.

## Configuration

- Override defaults via the app’s ConfigMap or your platform’s override mechanism.
- Common options: `node-feature-discovery.enabled`, `kmm.enabled`, `multus.enabled`, `installdefaultNFDRule`, and controller resource limits (see [Resource Configuration](https://instinct.docs.amd.com/projects/network-operator/en/main/installation/kubernetes-helm.html#resource-configuration)).

## Installing With AMD GPU Operator

If both AMD GPU Operator and AMD Network Operator run on the same cluster, use AMD’s combined installation guide to align NFD/KMM and avoid conflicts: [Installation of GPU Operator and Network Operator together](https://instinct.docs.amd.com/projects/network-operator/en/main/installation/kubernetes-helm-operators.html).
