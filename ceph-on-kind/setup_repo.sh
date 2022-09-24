cp -r ~/git/organizations/mesosphere/kommander-applications/services/rook-ceph ./repo/services/
cp -r ~/git/organizations/mesosphere/kommander-applications/services/rook-ceph-cluster ./repo/services/
cp rook-ceph-cluster-values.yaml ./repo/services/rook-ceph-cluster/1.10.1/defaults/cm.yaml
k apply -f rook-ceph-helmrepo.yaml

