# Zippyra Observability Stack — Deployment Guide

## Prerequisites

- Kubernetes cluster (EKS 1.29+)
- `kubectl` configured for the target cluster
- `helm` v3 installed

## Deploy observability stack

```bash
# 1. Create monitoring namespace
kubectl apply -f namespace.yaml

# 2. Install kube-prometheus-stack via Helm
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update
helm install kube-prometheus-stack prometheus-community/kube-prometheus-stack \
  -n monitoring -f prometheus/values.yaml

# 3. Apply custom alerts
kubectl apply -f prometheus/alerts.yaml

# 4. Deploy OpenTelemetry Collector
kubectl apply -f otel/otel-collector.yaml

# 5. Deploy Jaeger
kubectl apply -f jaeger/jaeger.yaml

# 6. Load Grafana dashboards
kubectl apply -f grafana/grafana-dashboards-configmap.yaml

# 7. Configure AlertManager routing
kubectl apply -f alertmanager/alertmanager.yaml
```

## Access UIs via port-forward

```bash
# Grafana (default: admin / from grafana-admin secret)
kubectl port-forward -n monitoring svc/grafana 3000:80

# Jaeger UI
kubectl port-forward -n monitoring svc/jaeger-query 16686:16686

# Prometheus UI
kubectl port-forward -n monitoring svc/kube-prometheus-stack-prometheus 9090:9090
```

## Verification

```bash
# Check all pods are running
kubectl get pods -n monitoring

# Verify Prometheus targets
# Open http://localhost:9090/targets — all zippyra services should be UP

# Verify Grafana dashboards
# Open http://localhost:3000 — 3 dashboards should be loaded

# Verify Jaeger traces
# Open http://localhost:16686 — select a service to view traces
```
