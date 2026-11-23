# Migrating from Gen2b to operator-go v0.13 Gen3

This release replaces the legacy per-role reconciler with operator-go v0.13's
`GenericReconciler`. It is a breaking migration and requires a planned outage.

## What changes

- All four roles are reconciled as StatefulSets. The API and alert roles were Deployments.
- StatefulSet selectors and governing Service names change. Gen3 uses a separate
  `<cluster>-<role>-<group>-headless` Service.
- Workload RBAC moves from the fixed `dolphinscheduler` identity to the
  cluster-specific `dolphinschedulercluster-<cluster>` ServiceAccount, Role, and RoleBinding.
- Status uses operator-go's generic conditions and role-group status fields.
- Database and S3 credentials are delivered from Secrets or SecretClass CSI volumes; they are
  no longer copied into ConfigMaps.
- DolphinScheduler 3.2.2 only supports path-style access when an S3 endpoint is configured. Set
  `clusterConfig.s3.connection.inline.pathStyle: true` (or set it on the referenced
  `S3Connection`); virtual-host style is rejected instead of being silently misrepresented.

The controller detects owned Gen2b StatefulSets, Deployments, incompatible headless client
Services, and fixed-name RBAC before it applies Gen3 resources. Its error contains exact,
namespace-qualified deletion commands. It never deletes these resources automatically.

## Upgrade procedure

1. Back up the DolphinScheduler database and the `DolphinschedulerCluster` object. Record each
   worker PVC's name, UID, and bound PV.
2. Stop submissions through the API and schedule downtime. Scale the operator Deployment to zero.
3. Scale the legacy workloads to zero and wait for their Pods to terminate.
4. Delete only the legacy workload controllers, incompatible client Services, and fixed-name
   RBAC identified by the controller's migration error. Do not delete worker PVCs.
5. Deploy the Gen3 operator and restore its Deployment replica count.
6. Wait for the database-init Job and all four role groups to become ready. Confirm that every
   worker claim retained its UID/PV binding and run a real workflow through the REST API.

The completed legacy database-init Job is recreated once during migration. A legacy Job does not
record which database name it initialized, so adopting it could incorrectly skip schema setup when
the database target changes at the same time as the operator upgrade. Gen3 records the resolved
product image and a credential-free database-target hash; changing either identity recreates even
a completed or failed Job so the idempotent schema upgrader runs against the intended database.
Active and failed Jobs also record a hash of the credentials Secret name and `resourceVersion`,
so correcting rotated credentials restarts the immutable Job without persisting credential data.
A completed Job is not rerun for a credentials-only rotation because its schema work already
succeeded.

The rendered role-group configuration and changes to the referenced ZooKeeper or generated
environment ConfigMap, database credentials Secret, S3Connection, and API OIDC credential Secrets
are included in the StatefulSet pod-template dependency hash. Their updates therefore roll the
affected Pods after the reference watch wakes the controller. Secret values are never included in
the hash; only their Kubernetes `resourceVersion` is used.

The S3 credential startup fragment remains local until operator-go's helper fails closed when a
CSI credential file is unreadable or empty. The public framework helper currently uses an
`export VAR="$(cat ...)"` command whose successful `export` status can mask a failed file read.

S3 and LDAP SecretClass credentials are read into process environment variables at container
startup. The current secret-operator/operator-go lifecycle does not provide an event-to-rollout
contract for SecretClass backend rotation, so rotate those credentials together with an explicit
restart of the affected role-group StatefulSets; hot rotation is not claimed by this migration.

## Maintainer acceptance

`make framework-upgrade-e2e` automates a fresh-cluster Gen2b to Gen3 to Gen2b round trip. It
records normalized resource snapshots, REST workflow evidence, database state, RBAC, Job
identity, and worker PVC data/identity under `target/upgrade-evidence`. The dependency charts and
test client image are digest locked. This is a destructive, long-running kind test and must use a
dedicated cluster.
