<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-04-06 | Updated: 2026-04-06 -->

# dolphinscheduler-operator

## Purpose
Manages Apache DolphinScheduler deployments on Kubernetes. Handles creation, configuration, and lifecycle management of DolphinScheduler cluster instances including master, worker, api, and alerter role components for distributed workflow scheduling.

## Key Files
| File | Description |
|------|-------------|
| `go.mod` | Go module dependencies |
| `Makefile` | Build and development commands |
| `PROJECT` | Kubebuilder project metadata |
| `Dockerfile` | Container image definition |

## Subdirectories
| Directory | Purpose |
|-----------|---------|
| `api/v1alpha1/` | Kubernetes CRD definitions (`DolphinschedulerCluster`) |
| `cmd/` | Operator entry point and main function |
| `config/` | Kubernetes manifests and kustomize configs |
| `deploy/` | Additional deployment manifests |
| `hack/` | Build and code generation scripts |
| `internal/common/` | Shared reconciliation helpers (config, container, workload, logging, vector) |
| `internal/controller/` | Main controller and role sub-controllers |
| `internal/controller/cluster/` | Cluster-level reconciliation (RBAC, init job, cluster resources) |
| `internal/controller/master/` | Master role reconciliation |
| `internal/controller/worker/` | Worker role reconciliation |
| `internal/controller/api/` | API server role reconciliation |
| `internal/controller/alerter/` | Alerter role reconciliation |
| `internal/security/` | Security and authentication helpers |
| `pkg/` | Shared utilities |
| `test/` | E2E test suites |

## For AI Agents

### Working In This Directory
- Standard Kubebuilder operator structure
- Uses `operator-go` framework (`github.com/zncdatadev/operator-go`) for reconciliation
- The single CRD is `DolphinschedulerCluster` (group `dolphinscheduler.kubedoop.dev`, version `v1alpha1`)
- Four roles: `master`, `worker`, `api`, `alerter` — each has its own sub-directory under `internal/controller/`
- Cluster-wide resources (RBAC, DB init job) are in `internal/controller/cluster/`
- Shared helpers for config generation, container spec, workload building, and log aggregation are in `internal/common/`
- Run `make test` for unit tests
- Run `make generate && make manifests` after changing API types
- Run `make deploy` to deploy to cluster

### Testing Requirements
- E2E tests in `test/e2e/`
- Requires a Kubernetes cluster for E2E testing (kind supported via `make kind-create`)
- ZooKeeper is a required external dependency for the cluster (`clusterConfig.zookeeperConfigMapName`)
- A database (PostgreSQL/MySQL) is also required (`clusterConfig.database`)

### Common Patterns
- Controllers in `internal/controller/`
- CRDs use `v1alpha1` API version
- Follows `operator-go` `GenericReconciler` pattern
- `DolphinschedulerClusterReconciler` delegates to `cluster.ClusterReconciler` which registers role reconcilers
- Each role reconciler is under its own package (e.g., `master`, `worker`, `api`, `alerter`)
- Configuration is generated via `internal/common/config.go` and mounted as ConfigMaps
- Vector log aggregation is supported via `internal/common/vector.go`
- Authentication (LDAP) is supported via `internal/security/`

## Dependencies

### Internal
- `../operator-go` — Shared operator framework (`github.com/zncdatadev/operator-go v0.12.6`)

### External
- `sigs.k8s.io/controller-runtime` — Kubernetes controller runtime
- `k8s.io/client-go` — Kubernetes client
- Apache ZooKeeper (external cluster dependency at runtime)
- PostgreSQL or MySQL (external cluster dependency at runtime)

<!-- MANUAL: -->
