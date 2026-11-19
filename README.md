# rancher/ali-operator

Ali operator is a Kubernetes CRD controller that controls cluster provisioning in Alibaba Container Service for Kubernetes using an AliClusterConfig defined by a Custom Resource Definition.

## Release

#### Chart releases (charts v2)

Once the images of a `v*` tag are published, the release workflow runs the
[Stage chart](.github/workflows/stage-chart.yaml) workflow. It stages the `rancher-ali-operator` and
`rancher-ali-operator-crd` charts in
the staging registry with
[cargueiro](https://github.com/rancher/cargueiro/blob/main/docs/CHART_OWNER.md) and opens a pull
request against the `staging` branch of `rancher/charts`. The charts come from `charts/ali-operator`
and `charts/ali-operator-crd`, prepared for the tag by
[prepare-charts.sh](.github/workflows/scripts/prepare-charts.sh); the chart version is worked out
from the `catalog.cattle.io/rancher-version` annotation, the app version and the versions already
released. The chart steps of the release workflow stay until the old chart path is retired.

A tag `v1.N.x` is for Rancher 2.N. Before the first tag of a new minor, update the
`catalog.cattle.io/rancher-version` and `kube-version` annotations in `charts/ali-operator/Chart.yaml`:
the workflow refuses a tag that does not match them, before anything is pushed. If it fails, fix the
cause and run it again on the tag with `gh workflow run stage-chart.yaml --ref <tag>`; what to do
after a push is in [When something fails](https://github.com/rancher/cargueiro/blob/main/docs/CHART_OWNER.md#when-something-fails).
