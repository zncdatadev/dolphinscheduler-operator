<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-04-06 | Updated: 2026-09-14 -->

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
| `internal/controller/` | Gen3 role catalog/handler, product config resolver, cluster extension, and tests |
| `internal/security/` | Security and authentication helpers |
| `pkg/util/` | Database and properties helpers |
| `test/` | E2E test suites |

## For AI Agents

### Working In This Directory
- Standard Kubebuilder operator structure
- Uses `operator-go` framework (`github.com/zncdatadev/operator-go`) for reconciliation
- The single CRD is `DolphinschedulerCluster` (group `dolphinscheduler.kubedoop.dev`, version `v1alpha1`)
- Four roles: `master`, `worker`, `api`, `alerter` — all are declared by `DolphinSchedulerRoleGroupHandler`
- `cmd/main.go` wires the CR directly to operator-go's `GenericReconciler`
- `internal/controller/cluster_extension.go` owns the cluster env ConfigMap, DB init Job, and Gen2b upgrade guard
- `internal/controller/product_config.go` supplies role defaults and the per-role-group derived config contribution
- The framework owns standard per-role-group ConfigMaps, Services, StatefulSets, PodDisruptionBudgets, and workload RBAC
- Run `make test` for unit tests
- Run `make lint` for the lint gate
- Run `make generate && make manifests` after changing API types
- Run `make deploy` to deploy to cluster

### Testing Requirements
- E2E tests in `test/e2e/`
- Requires a Kubernetes cluster for E2E testing (kind supported via `make kind-create`)
- ZooKeeper is a required external dependency for the cluster (`clusterConfig.zookeeperConfigMapName`)
- A database (PostgreSQL/MySQL) is also required (`clusterConfig.database`)

### Common Patterns
- One `GenericReconciler` in `cmd/main.go`; product-specific reconciliation stays behind the role handler and cluster extension seams
- CRDs use `v1alpha1` API version
- Follows `operator-go` `GenericReconciler` pattern
- `DolphinSchedulerRoleGroupHandler` implements both `RoleProvider` and `RoleGroupHandler`
- Configuration defaults are folded by operator-go and rendered by the handler from `RoleGroupBuildContext.MergedConfig`
- Per-CR extensions are registered through an `ExtensionRegistry`; never store CR-specific state on the handler
- Workload Pods use the framework-derived per-cluster ServiceAccount and RBAC
- LDAP and OIDC authentication are resolved through `internal/security/`

## Dependencies

### Internal
- `operator-go` — Shared Gen3 operator framework (`github.com/zncdatadev/operator-go v0.13.0`)

### External
- `sigs.k8s.io/controller-runtime` — Kubernetes controller runtime
- `k8s.io/client-go` — Kubernetes client
- Apache ZooKeeper (external cluster dependency at runtime)
- PostgreSQL or MySQL (external cluster dependency at runtime)

<!-- MANUAL: -->
