#!/bin/bash

echo "=== 检查 image-reflector-controller Deployment ==="
kubectl get deployment image-reflector-controller -n kommander-flux -o yaml | grep -A 5 -B 5 priorityClassName

echo ""
echo "=== 检查 image-automation-controller Deployment ==="
kubectl get deployment image-automation-controller -n kommander-flux -o yaml | grep -A 5 -B 5 priorityClassName

echo ""
echo "=== 检查 HelmRelease 状态 ==="
kubectl get helmrelease kommander-flux -n kommander-flux -o yaml | grep -A 10 -B 5 "status:"

echo ""
echo "=== 检查 ConfigMap 内容 ==="
kubectl get configmap kommander-flux-overrides -n kommander -o yaml | grep -A 20 imageReflectorController

echo ""
echo "=== 检查 Pod 的完整 spec ==="
kubectl get pod -n kommander-flux -l app=image-reflector-controller -o yaml | grep -A 3 -B 3 priorityClassName
