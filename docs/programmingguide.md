# Developer Reference

To locally build this project from the root of this repository:

```sh
CGO_ENABLED=0 GOOS=linux go build -mod=mod github.com/Azure/aks-periscope/cmd/aks-periscope
```

## Automated Tests

See [this guide](testing.md) for running automated tests in a CI or development environment.

## Manual Testing

See [Dynamic image overlay](../deployment/overlays/dynamic-image/README.md) to deploy changes to your cluster.
