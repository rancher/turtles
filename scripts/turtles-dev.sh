#!/usr/bin/env bash

RANCHER_HOSTNAME=$1
if [ -z "$RANCHER_HOSTNAME" ]
then
    echo "You must pass a rancher host name"
    exit 1
fi

BASEDIR=$(dirname "$0")

kind create cluster --config "$BASEDIR/kind-cluster-with-extramounts.yaml"

kubectl rollout status deployment coredns -n kube-system --timeout=90s

kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v1.5.1/cert-manager.crds.yaml

helm install cert-manager jetstack/cert-manager --namespace cert-manager --create-namespace --version v1.5.1

kubectl rollout status deployment cert-manager -n cert-manager --timeout=90s

export EXP_CLUSTER_RESOURCE_SET=true
clusterctl init -i docker

kubectl create namespace cattle-system

helm install rancher rancher-stable/rancher --namespace cattle-system --set bootstrapPassword=admin --set replicas=1  --set hostname="$RANCHER_HOSTNAME" --set 'extraEnv[0].name=CATTLE_FEATURES' --set 'extraEnv[0].value=embedded-cluster-api=false' --set global.cattle.psp.enabled=false --version 2.7.5

kubectl rollout status deployment rancher -n cattle-system --timeout=180s

tilt up

