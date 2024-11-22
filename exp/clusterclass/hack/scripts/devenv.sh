#!/usr/bin/env bash

BASEDIR=$(dirname "$0")

kind create cluster --config "$BASEDIR/kind-cluster-with-extramounts.yaml"

kubectl rollout status deployment coredns -n kube-system --timeout=90s

export EXP_CLUSTER_RESOURCE_SET=true
export CLUSTER_TOPOLOGY=true
clusterctl init -i docker

tilt up
