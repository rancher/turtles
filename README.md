# CAPI + Rancher = :cupid:

A proof-of-concept project looking at various aspects of making Rancher :heart: Cluster API.

> As a proof-of-concept this is doesn't in anyway indicate a future Rancher strategy and you use this at your own risk, there is no support! The code may be a bit rubbish as well.

## What is covered in this PoC?

Currently this project has the following functionality:

- Automatically import CAPI created cluster into Rancher

## How to use this?

Instructions coming soon :)

## How to contribute?

Instructions coming soon :)

Create **tilt-settings.yaml** like this:

```yaml
{
    "k8s_context": "k3d-rancher-test",
    "default_registry": "ghcr.io/richardcase",
    "debug": {
        "turtles": {
            "continue": true,
            "port": 40000
        }
    }
}
```
