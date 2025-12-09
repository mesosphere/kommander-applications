#!/bin/bash

echo "=== 步骤 1: 检查 Deployment 配置 ==="
echo "检查 image-reflector-controller Deployment 是否包含 priorityClassName..."
kubectl get deployment image-reflector-controller -n kommander-flux -o jsonpath='{.spec.template.spec.priorityClassName}' && echo "" || echo "未找到 priorityClassName"

echo ""
echo "=== 步骤 2: 检查当前 Pod 状态 ==="
kubectl get pods -n kommander-flux -o custom-columns=NAME:.metadata.name,PRIORITY:.spec.priorityClassName | grep image

echo ""
echo "=== 步骤 3: 如果 Deployment 已更新，删除 Pod 强制重新创建 ==="
read -p "是否删除 image-reflector-controller Pod 以强制重新创建？(y/n) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]
then
    echo "正在删除 Pod..."
    kubectl delete pod -n kommander-flux -l app=image-reflector-controller
    echo "等待 10 秒后检查新 Pod..."
    sleep 10
    echo ""
    echo "=== 步骤 4: 验证修复 ==="
    kubectl get pods -n kommander-flux -o custom-columns=NAME:.metadata.name,PRIORITY:.spec.priorityClassName | grep image
else
    echo "跳过 Pod 删除"
fi

echo ""
echo "=== 如果仍然没有修复，检查 HelmRelease 状态 ==="
kubectl get helmrelease kommander-flux -n kommander-flux
