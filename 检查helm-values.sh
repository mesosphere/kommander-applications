#!/bin/bash

echo "=== 检查 ConfigMap 内容 ==="
echo "1. 默认配置 (kommander-flux-2.7.2-config-defaults):"
kubectl get configmap kommander-flux-2.7.2-config-defaults -n kommander -o yaml | grep -A 15 imageReflectorController

echo ""
echo "2. 覆盖配置 (kommander-flux-overrides):"
kubectl get configmap kommander-flux-overrides -n kommander -o yaml | grep -A 15 imageReflectorController

echo ""
echo "=== 检查 Helm Release 的实际 values ==="
kubectl get secret -n kommander -l owner=helm,name=kommander-flux -o jsonpath='{.items[0].metadata.name}' 2>/dev/null
SECRET_NAME=$(kubectl get secret -n kommander -l owner=helm,name=kommander-flux -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
if [ -n "$SECRET_NAME" ]; then
    echo "找到 Helm secret: $SECRET_NAME"
    echo "检查 values..."
    kubectl get secret "$SECRET_NAME" -n kommander -o jsonpath='{.data.release}' | base64 -d | gunzip 2>/dev/null | python3 -m json.tool 2>/dev/null | grep -A 20 imageReflectorController || echo "无法解析 Helm release"
else
    echo "未找到 Helm secret"
fi

echo ""
echo "=== 比较两个控制器的 Deployment ==="
echo "image-automation-controller:"
kubectl get deployment image-automation-controller -n kommander-flux -o jsonpath='{.spec.template.spec.priorityClassName}' && echo "" || echo "无"

echo "image-reflector-controller:"
kubectl get deployment image-reflector-controller -n kommander-flux -o jsonpath='{.spec.template.spec.priorityClassName}' && echo "" || echo "无"
