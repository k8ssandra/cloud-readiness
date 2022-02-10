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
kubectl delete crd servicemonitors.monitoring.coreos.com
kubectl delete crd stargates.stargate.k8ssandra.io
kubectl delete crd thanosrulers.monitoring.coreos.com
kubectl delete crd tlsoptions.traefik.containo.us
kubectl delete crd tlsstores.traefik.containo.us
kubectl delete crd traefikservices.traefik.containo.us
kubectl delete crd serverstransports.traefik.containo.us

backendconfigs.cloud.google.com                  2022-02-07T22:49:06Z
capacityrequests.internal.autoscaling.gke.io     2022-02-07T22:48:54Z
cassandrabackups.medusa.k8ssandra.io             2022-02-08T22:09:22Z
cassandradatacenters.cassandra.datastax.com      2022-02-08T22:09:26Z
cassandrarestores.medusa.k8ssandra.io            2022-02-08T22:09:22Z
cassandratasks.control.k8ssandra.io              2022-02-08T22:09:26Z
certificaterequests.cert-manager.io              2022-02-08T21:50:07Z
certificates.cert-manager.io                     2022-02-08T21:50:08Z
challenges.acme.cert-manager.io                  2022-02-08T21:50:09Z
clientconfigs.config.k8ssandra.io                2022-02-08T22:09:22Z
clusterissuers.cert-manager.io                   2022-02-08T21:50:12Z
frontendconfigs.networking.gke.io                2022-02-07T22:49:07Z
issuers.cert-manager.io                          2022-02-08T21:50:13Z
k8ssandraclusters.k8ssandra.io                   2022-02-08T22:09:23Z
managedcertificates.networking.gke.io            2022-02-07T22:48:57Z
orders.acme.cert-manager.io                      2022-02-08T21:50:15Z
reapers.reaper.k8ssandra.io                      2022-02-08T22:09:24Z
replicatedsecrets.replication.k8ssandra.io       2022-02-08T22:09:24Z
serviceattachments.networking.gke.io             2022-02-07T22:49:07Z
servicenetworkendpointgroups.networking.gke.io   2022-02-07T22:49:07Z
stargates.stargate.k8ssandra.io                  2022-02-08T22:09:25Z
updateinfos.nodemanagement.gke.io                2022-02-07T22:49:00Z
