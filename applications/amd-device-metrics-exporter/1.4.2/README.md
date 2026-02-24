# AMD Device Metrics Exporter

This application deploys the **AMD Device Metrics Exporter** on Kubernetes via Flux (HelmRepository + HelmRelease), using the official ROCm Helm chart.

## Overview

The AMD Device Metrics Exporter collects telemetry from **AMD GPUs** (and optionally **NICs**) and exposes it in **Prometheus** format:

- Temperature, utilization, memory usage, power consumption
- PCIe bandwidth and other performance metrics
- Optional **ServiceMonitor** integration for Prometheus Operator
- **DRA (Dynamic Resource Allocation)** claim association on Kubernetes 1.34+ (beta)

It can run standalone (e.g. for clusters that already have the device plugin or GPU Operator elsewhere) or alongside the AMD GPU Operator. The GPU Operator can also embed metrics via its DeviceConfig; this app provides a dedicated deployment of the exporter with its own Helm values.

## Documentation

| Resource | Link |
|----------|------|
| **Installation (Helm)** | [Kubernetes (Helm) installation — AMD Device Metrics Exporter](https://instinct.docs.amd.com/projects/device-metrics-exporter/en/latest/installation/kubernetes-helm.html) |
| **Prometheus / Grafana** | [Prometheus and Grafana integration](https://instinct.docs.amd.com/projects/device-metrics-exporter/en/latest/integrations/prometheus-grafana.html) |
| **Configuration** | [Configuration](https://instinct.docs.amd.com/projects/device-metrics-exporter/en/latest/configuration/) (values, ConfigMap, DRA) |
| **Troubleshooting** | [Troubleshooting Device Metrics Exporter](https://instinct.docs.amd.com/projects/device-metrics-exporter/en/latest/configuration/troubleshooting.html) |
| **GitHub** | [ROCm/device-metrics-exporter](https://github.com/ROCm/device-metrics-exporter) |

## Prerequisites

- Kubernetes cluster **v1.29.0 or later**
- **ROCm** stack on GPU nodes (e.g. via AMD GPU Operator or pre-installed)
- Ubuntu **22.04 or later** on nodes where the exporter runs
- For DRA: Kubernetes **1.34+** and [AMD GPU DRA driver](https://github.com/ROCm/k8s-gpu-dra-driver) if using DRA claims

## How This App Deploys It

- **Helm repository:** `https://rocm.github.io/device-metrics-exporter`
- **Chart:** `device-metrics-exporter-charts`
- **Version:** `v1.4.2`
- **Release name:** `amd-device-metrics-exporter`
- **Target namespace:** Workspace release namespace (`releaseNamespace`)

Defaults in the app enable **GPU** monitoring and **ServiceMonitor** with `prometheus.kommander.d2iq.io/select: "true"` for Kommander Prometheus. NIC monitoring is disabled by default; set `monitor.resources.nic: true` in values if needed.

## Configuration

- **platform:** `k8s` (default)
- **monitor.resources.gpu** / **monitor.resources.nic** – Enable GPU and/or NIC metrics.
- **image.repository** / **image.tag** – Exporter image (default `docker.io/rocm/device-metrics-exporter:v1.4.2`).
- **service.type** – `ClusterIP` or `NodePort`.
- **serviceMonitor** – Interval, labels, relabelings for Prometheus Operator (defaults include Kommander scrape label).

Override via the app’s ConfigMap or your platform’s override mechanism. See [Kubernetes (Helm) installation](https://instinct.docs.amd.com/projects/device-metrics-exporter/en/latest/installation/kubernetes-helm.html) for the full values schema.
