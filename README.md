Project Resource Collection API Extension Server
================================================

Dev Setup
---------

```
kubectl apply -f examples/manifest.yaml
kubectl port-forward port-forward 6666:6666 &
KUBECONFIG=/path/to/kubeconfig go run main.go
```
