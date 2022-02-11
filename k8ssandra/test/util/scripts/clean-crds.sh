#!/bin/bash
# This script deletes all CRDS that are installed by k8ssandra. Note that helm
# uninstall does not remove CRDs.

kubectl delete crd alertmanagerconfigs.monitoring.coreos.com
kubectl delete crd alertmanagers.monitoring.coreos.com
kubectl delete crd cassandrabackups.cassandra.k8ssandra.io
kubectl delete crd cassandradatacenters.cassandra.datastax.com
kubectl delete crd cassandrarestores.cassandra.k8ssandra.io
kubectl delete crd cassandrabackups.medusa.k8ssandra.io
kubectl delete crd cassandrarestores.medusa.k8ssandra.io
kubectl delete crd cassandratasks.control.k8ssandra.io
kubectl delete crd clientconfigs.config.k8ssandra.io
kubectl delete crd grafanadashboards.integreatly.org
kubectl delete crd grafanadatasources.integreatly.org
kubectl delete crd grafanas.integreatly.org
kubectl delete crd ingressroutes.traefik.containo.us
kubectl delete crd ingressroutetcps.traefik.containo.us
kubectl delete crd ingressrouteudps.traefik.containo.us
kubectl delete crd k8ssandraclusters.k8ssandra.io
kubectl delete crd middlewares.traefik.containo.us
kubectl delete crd podmonitors.monitoring.coreos.com
kubectl delete crd probes.monitoring.coreos.com
kubectl delete crd prometheuses.monitoring.coreos.com
kubectl delete crd prometheusrules.monitoring.coreos.com
kubectl delete crd reapers.reaper.cassandra-reaper.io
kubectl delete crd replicatedsecrets.replication.k8ssandra.io
kubectl delete crd servicemonitors.monitoring.coreos.com
kubectl delete crd stargates.stargate.k8ssandra.io
kubectl delete crd thanosrulers.monitoring.coreos.com
kubectl delete crd tlsoptions.traefik.containo.us
kubectl delete crd tlsstores.traefik.containo.us
kubectl delete crd traefikservices.traefik.containo.us
kubectl delete crd serverstransports.traefik.containo.us

