# An example clean script for xtra cleanup required
kubectx gke_community-ecosystem_us-central1_dev-bootz100
kubectl delete k8ssandracluster/bootz-k8c-cluster -n bootz
./clean-crds.sh
./clean-ns.sh

kubectx gke_community-ecosystem_us-central1_dev-bootz101
kubectl delete k8ssandracluster/bootz-k8c-cluster -n bootz
./clean-crds.sh
./clean-ns.sh

kubectx gke_community-ecosystem_us-central1_dev-bootz102
kubectl delete k8ssandracluster/bootz-k8c-cluster -n bootz
./clean-crds.sh
./clean-ns.sh