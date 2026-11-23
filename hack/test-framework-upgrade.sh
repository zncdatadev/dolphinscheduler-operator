#!/usr/bin/env bash
# Destructive acceptance test confined to an empty, explicitly selected kind cluster.
set -euo pipefail

: "${KUBECONFIG:?Set KUBECONFIG to the disposable kind cluster}"
: "${CHAINSAW_CLUSTER:?Set the disposable kind cluster name}"
: "${UPGRADE_BASELINE_DIR:?Provide a checkout of the pre-framework operator}"
: "${UPGRADE_BASELINE_IMAGE:?Build the pre-framework image first}"
: "${UPGRADE_DEPENDENCY_LOCK:?Provide the verified dependency chart lock file}"
: "${UPGRADE_PRODUCT_IMAGE:?Provide the digest-pinned DolphinScheduler product image}"
: "${IMG:?Build the framework image first}"

root=$(cd "$(dirname "$0")/.." && pwd)
product_version=${PRODUCT_VERSION:-3.2.2}
timeout_seconds=${UPGRADE_TIMEOUT_SECONDS:-900}
namespace=dolphinscheduler-framework-upgrade
operator_namespace=dolphinscheduler-operator-system
operator_deployment=dolphinscheduler-operator-controller-manager
cluster=upgrade
evidence_root=${UPGRADE_EVIDENCE_DIR:-$root/upgrade-evidence}
mkdir -p "$evidence_root"
evidence_root=$(cd "$evidence_root" && pwd)
evidence=$(mktemp -d "$evidence_root/run.XXXXXX")
scratch=$(mktemp -d)
echo "Migration evidence: $evidence"

k() { kubectl -n "$namespace" "$@"; }

finish() {
    local result=$?
    if (( result != 0 )); then
        k get dolphinschedulercluster,sts,deploy,pod,pvc,svc,pdb,cm,sa,role,rolebinding,job \
            -o yaml > "$evidence/failure-resources.yaml" 2>&1 || true
        k get events --sort-by=.lastTimestamp > "$evidence/failure-events.txt" 2>&1 || true
        kubectl -n "$operator_namespace" logs "deployment/$operator_deployment" \
            --tail=300 > "$evidence/failure-operator.log" 2>&1 || true
    fi
    rm -rf "$scratch"
}
trap finish EXIT

for tool in kubectl kind python3; do
    command -v "$tool" >/dev/null 2>&1 || {
        echo "$tool is required" >&2
        exit 1
    }
done
test -x "$root/bin/kustomize" || {
    echo "$root/bin/kustomize is required; run the Make target rather than this script directly" >&2
    exit 1
}
test "$(kubectl config current-context)" = "kind-$CHAINSAW_CLUSTER"

existing_clusters=$(kubectl get dolphinschedulerclusters -A -o name 2>/dev/null || true)
if [[ -n "$existing_clusters" ]]; then
    echo "Use an empty kind cluster: existing DolphinschedulerClusters would be affected." >&2
    exit 1
fi
if kubectl get namespace "$namespace" >/dev/null 2>&1; then
    echo "Namespace $namespace already exists; inspect the previous run before retrying." >&2
    exit 1
fi
test -f "$UPGRADE_DEPENDENCY_LOCK"
cp "$UPGRADE_DEPENDENCY_LOCK" "$evidence/dependency-charts.sha256"
kubectl -n kubedoop-operators get deployment,statefulset,pod -o json \
    > "$evidence/dependency-runtime.json"

wait_for() {
    local description=$1
    shift
    local deadline=$((SECONDS + timeout_seconds))
    until "$@"; do
        if (( SECONDS >= deadline )); then
            echo "Timed out: $description" >&2
            k get sts,deploy,pod,pvc,svc -o wide || true
            k get events --sort-by=.lastTimestamp || true
            return 1
        fi
        sleep 5
    done
}

statefulset_ready() {
    k get sts "$1" -o json | python3 -c '
import json, sys
s = json.load(sys.stdin)
desired = s["spec"].get("replicas", 1)
status = s.get("status", {})
assert status.get("readyReplicas", 0) == desired
assert status.get("observedGeneration", 0) >= s["metadata"]["generation"]
' 2>/dev/null
}

deployment_ready() {
    k get deployment "$1" -o json | python3 -c '
import json, sys
d = json.load(sys.stdin)
desired = d["spec"].get("replicas", 1)
status = d.get("status", {})
assert status.get("readyReplicas", 0) == desired
assert status.get("observedGeneration", 0) >= d["metadata"]["generation"]
' 2>/dev/null
}

pod_ready() {
    k get pod "$1" -o json | python3 -c '
import json, sys
p = json.load(sys.stdin)
assert p.get("status", {}).get("phase") == "Running"
conditions = {item["type"]: item["status"] for item in p.get("status", {}).get("conditions", [])}
assert conditions.get("Ready") == "True"
' 2>/dev/null
}

znode_ready() {
    k get configmap upgrade-znode -o json | python3 -c '
import json, sys
assert json.load(sys.stdin).get("data")
' 2>/dev/null
}

job_complete() {
    k get job "$1" -o json | python3 -c '
import json, sys
j = json.load(sys.stdin)
assert j.get("status", {}).get("succeeded", 0) >= 1
' 2>/dev/null
}

render_operator() {
    local source=$1 image=$2 output=$3
    local render
    render=$(mktemp -d "$scratch/render.XXXXXX")
    cp -R "$source/config" "$render/config"
    (cd "$render/config/manager" && "$root/bin/kustomize" edit set image "controller=$image")
    "$root/bin/kustomize" build "$render/config/default" > "$output"
}

deploy_operator() {
    kubectl apply --server-side=true --force-conflicts \
        --field-manager=framework-upgrade-e2e -f "$1"
    kubectl -n "$operator_namespace" rollout status \
        "deployment/$operator_deployment" --timeout=300s
}

stop_operator() {
    kubectl -n "$operator_namespace" scale "deployment/$operator_deployment" --replicas=0
    kubectl -n "$operator_namespace" wait --for=delete pod \
        -l control-plane=controller-manager --timeout=180s
}

no_pods_with_prefix() {
    local prefix=$1
    local matches
    matches=$(k get pods -o name | grep -E "^pod/${prefix}" || true)
    [[ -z "$matches" ]]
}

delete_workload() {
    local kind=$1 name=$2
    if ! k get "$kind/$name" >/dev/null 2>&1; then
        return
    fi
    if [[ "$kind" == statefulset && "$name" == upgrade-worker-default ]]; then
        # Explicitly retain the data claim across both destructive shape changes.
        k patch "$kind/$name" --type merge -p \
            '{"spec":{"persistentVolumeClaimRetentionPolicy":{"whenDeleted":"Retain","whenScaled":"Retain"}}}'
    fi
    k scale "$kind/$name" --replicas=0
    wait_for "$kind/$name pods deleted" no_pods_with_prefix "$name-"
    k delete "$kind/$name" --wait=true
}

replace_product_workloads() {
    stop_operator
    # Stop the API first so no new workflows enter during the destructive
    # workload-shape transition, then stop the schedulers and executors.
    for role in api master worker alert; do
        local name="$cluster-$role-default"
        delete_workload statefulset "$name"
        delete_workload deployment "$name"
        # Recreate the service topology too: Gen2b uses the client Service itself
        # as headless for master/worker, while Gen3 adds a distinct -headless Service.
        k delete service "$name" "$name-headless" "$name-metrics" --ignore-not-found
    done
}

delete_namespaced_rbac() {
    local name=$1
    k delete serviceaccount "$name" --ignore-not-found
    k delete role "$name" --ignore-not-found
    k delete rolebinding "$name" --ignore-not-found
}

assert_workload_shape() {
    local phase=$1
    k get statefulset,deployment -o json > "$scratch/$phase-workloads.json"
    python3 - "$scratch/$phase-workloads.json" "$phase" <<'PY'
import json, pathlib, sys

items = json.loads(pathlib.Path(sys.argv[1]).read_text())["items"]
phase = sys.argv[2]
present = {(item["kind"], item["metadata"]["name"]) for item in items}
framework = phase == "after"
for role in ("master", "worker", "api", "alert"):
    name = f"upgrade-{role}-default"
    expected_sts = framework or role in {"master", "worker"}
    expected_deploy = not framework and role in {"api", "alert"}
    assert (("StatefulSet", name) in present) == expected_sts, (phase, name, "StatefulSet")
    assert (("Deployment", name) in present) == expected_deploy, (phase, name, "Deployment")
    assert not ({("StatefulSet", name), ("Deployment", name)} <= present), (phase, name, "double workload")
PY
}

wait_product_ready() {
    local phase=$1
    if [[ "$phase" == after ]]; then
        for role in master worker api alert; do
            wait_for "$phase $role ready" statefulset_ready "$cluster-$role-default"
        done
    else
        for role in master worker; do
            wait_for "$phase $role ready" statefulset_ready "$cluster-$role-default"
        done
        for role in api alert; do
            wait_for "$phase $role ready" deployment_ready "$cluster-$role-default"
        done
    fi
    assert_workload_shape "$phase"
}

api_raw() {
    # The product image deliberately does not promise curl.  A digest-pinned,
    # inert client pod makes every API assertion independent of product tooling.
    k exec pod/upgrade-api-client -c curl -- curl --silent --show-error --fail \
        --connect-timeout 5 --max-time 30 "$@"
}

api_origin=http://upgrade-api-default:12345
api_base=$api_origin/dolphinscheduler

api_ready() {
    local health response
    # Upstream defaults to the DolphinScheduler servlet context, while the
    # operator's historical probe uses the root path.  Accept either health
    # location, but the business calls below always exercise the real API.
    for health in \
        "$api_base/actuator/health/readiness" \
        "$api_origin/actuator/health/readiness"; do
        response=$(api_raw "$health" 2>/dev/null) || continue
        if python3 -c '
import json, sys
assert json.load(sys.stdin).get("status") == "UP"
' <<<"$response" 2>/dev/null; then
            return 0
        fi
    done
    return 1
}

assert_api_success() {
    python3 -c '
import json, sys
payload = json.load(sys.stdin)
if payload.get("code") != 0:
    raise SystemExit(
        "DolphinScheduler API error {}: {}".format(payload.get("code"), payload.get("msg"))
    )
' <<<"$1"
}

json_path() {
    python3 -c '
import json, sys
value = json.load(sys.stdin)
for part in sys.argv[1].split("."):
    value = value[part]
print(value)
' "$2" <<<"$1"
}

login() {
    local username=$1 password=$2 response
    response=$(api_raw -X POST \
        --data-urlencode "userName=$username" \
        --data-urlencode "userPassword=$password" \
        "$api_base/login")
    assert_api_success "$response"
    json_path "$response" data.sessionId
}

acceptance_user=framework_upgrade
acceptance_password=framework-upgrade-123
project_name=framework_upgrade_acceptance
workflow_name=framework_upgrade_shell
task_code=918273645012345
project_code=
workflow_code=
session_id=

create_acceptance_workflow() {
    local admin_session response tenant_id
    admin_session=$(login admin dolphinscheduler123)
    # The product image runs as the existing Linux user "kubedoop".  Using a
    # matching DolphinScheduler tenant makes the shell task independent of
    # sudo/useradd behavior inside the restricted Gen3 container.
    response=$(api_raw -X POST -H "sessionId: $admin_session" \
        --data-urlencode tenantCode=kubedoop \
        --data-urlencode queueId=1 \
        --data-urlencode description=operator-framework-upgrade-acceptance \
        "$api_base/tenants")
    assert_api_success "$response"
    tenant_id=$(json_path "$response" data.id)

    response=$(api_raw -X POST -H "sessionId: $admin_session" \
        --data-urlencode "userName=$acceptance_user" \
        --data-urlencode "userPassword=$acceptance_password" \
        --data-urlencode "tenantId=$tenant_id" \
        --data-urlencode queue=default \
        --data-urlencode email=framework-upgrade@example.invalid \
        --data-urlencode state=1 \
        "$api_base/users/create")
    assert_api_success "$response"

    session_id=$(login "$acceptance_user" "$acceptance_password")
    response=$(api_raw -X POST -H "sessionId: $session_id" \
        --data-urlencode "projectName=$project_name" \
        --data-urlencode description=operator-framework-upgrade-acceptance \
        "$api_base/projects")
    assert_api_success "$response"
    project_code=$(json_path "$response" data.code)

    local task_json relation_json
    task_json=$(printf '[{"code":%s,"name":"migration-shell","version":1,"description":"","delayTime":0,"taskType":"SHELL","taskParams":{"resourceList":[],"localParams":[],"rawScript":"echo framework-upgrade-continuity","dependence":{},"conditionResult":{"successNode":[],"failedNode":[]},"waitStartTimeout":{},"switchResult":{}},"flag":"YES","taskPriority":"MEDIUM","workerGroup":"default","failRetryTimes":0,"failRetryInterval":1,"timeoutFlag":"CLOSE","timeoutNotifyStrategy":null,"timeout":0,"environmentCode":-1}]' "$task_code")
    relation_json=$(printf '[{"name":"","preTaskCode":0,"preTaskVersion":0,"postTaskCode":%s,"postTaskVersion":1,"conditionType":0,"conditionParams":"{}"}]' "$task_code")
    response=$(api_raw -X POST -H "sessionId: $session_id" \
        --data-urlencode "name=$workflow_name" \
        --data-urlencode description=operator-framework-upgrade-acceptance \
        --data-urlencode globalParams='[]' \
        --data-urlencode locations='[]' \
        --data-urlencode timeout=0 \
        --data-urlencode "taskRelationJson=$relation_json" \
        --data-urlencode "taskDefinitionJson=$task_json" \
        "$api_base/projects/$project_code/process-definition")
    assert_api_success "$response"
    workflow_code=$(json_path "$response" data.code)

    response=$(api_raw -X POST -H "sessionId: $session_id" \
        --data-urlencode releaseState=ONLINE \
        "$api_base/projects/$project_code/process-definition/$workflow_code/release")
    assert_api_success "$response"
    printf '%s\n' "$project_code" > "$evidence/project-code.txt"
    printf '%s\n' "$workflow_code" > "$evidence/workflow-code.txt"
}

assert_api_field() {
    local response=$1 path=$2 expected=$3
    assert_api_success "$response"
    python3 -c '
import json, sys
payload = json.load(sys.stdin)
value = payload
for part in sys.argv[1].split("."):
    value = value[part]
assert str(value) == sys.argv[2], (sys.argv[1], value)
' "$path" "$expected" <<<"$response"
}

verify_acceptance_workflow() {
    project_code=$(<"$evidence/project-code.txt")
    workflow_code=$(<"$evidence/workflow-code.txt")
    session_id=$(login "$acceptance_user" "$acceptance_password")
    local response
    response=$(api_raw -H "sessionId: $session_id" "$api_base/projects/$project_code")
    assert_api_field "$response" data.name "$project_name"
    response=$(api_raw -H "sessionId: $session_id" \
        "$api_base/projects/$project_code/process-definition/$workflow_code")
    assert_api_field "$response" data.processDefinition.name "$workflow_name"
}

submit_workflow() {
    local phase=$1 response
    response=$(api_raw -X POST -H "sessionId: $session_id" \
        --data-urlencode "processDefinitionCode=$workflow_code" \
        --data-urlencode scheduleTime= \
        --data-urlencode failureStrategy=END \
        --data-urlencode warningType=NONE \
        --data-urlencode processInstancePriority=MEDIUM \
        --data-urlencode workerGroup=default \
        --data-urlencode tenantCode=kubedoop \
        "$api_base/projects/$project_code/executors/start-process-instance") || return 1
    printf '%s\n' "$response" > "$scratch/$phase-start.json"
    assert_api_success "$response"
}

workflow_instances_succeeded() {
    local expected=$1 response task_response instance_id
    local -a instance_ids
    response=$(api_raw --get -H "sessionId: $session_id" \
        --data-urlencode "processDefineCode=$workflow_code" \
        --data-urlencode pageNo=1 \
        --data-urlencode pageSize=100 \
        "$api_base/projects/$project_code/process-instances") || return 1
    printf '%s\n' "$response" > "$scratch/workflow-instances.json"
    python3 -c '
import json, sys
payload = json.load(sys.stdin)
assert payload.get("code") == 0, payload.get("msg")
items = payload["data"]["totalList"]
expected = int(sys.argv[1])
assert payload["data"]["total"] == expected
assert len(items) == expected
assert all(item["state"] == "SUCCESS" for item in items), [item["state"] for item in items]
' "$expected" <<<"$response" 2>/dev/null || return 1

    mapfile -t instance_ids < <(python3 -c '
import json, sys
for item in json.load(sys.stdin)["data"]["totalList"]:
    print(item["id"])
' <<<"$response")
    [[ ${#instance_ids[@]} -eq $expected ]] || return 1

    # A successful workflow state alone is not enough: query every process
    # instance and prove that its single Shell task reached SUCCESS as well.
    # Keep the raw task responses as durable acceptance evidence.
    : > "$scratch/workflow-task-instances.jsonl"
    for instance_id in "${instance_ids[@]}"; do
        task_response=$(api_raw -H "sessionId: $session_id" \
            "$api_base/projects/$project_code/process-instances/$instance_id/tasks") || return 1
        printf '%s\n' "$task_response" >> "$scratch/workflow-task-instances.jsonl"
        python3 -c '
import json, sys
payload = json.load(sys.stdin)
assert payload.get("code") == 0, payload.get("msg")
data = payload["data"]
tasks = data["taskList"]
assert data["processInstanceState"] == "SUCCESS", data["processInstanceState"]
assert len(tasks) == 1, len(tasks)
assert tasks[0]["taskType"] == "SHELL", tasks[0]["taskType"]
assert tasks[0]["state"] == "SUCCESS", tasks[0]["state"]
' <<<"$task_response" 2>/dev/null || return 1
    done
}

run_workflow() {
    local phase=$1 expected=$2
    wait_for "$phase workflow submission accepted" submit_workflow "$phase"
    cp "$scratch/$phase-start.json" "$evidence/$phase-workflow-start.json"
    wait_for "$phase workflow run $expected succeeded" workflow_instances_succeeded "$expected"
    cp "$scratch/workflow-instances.json" "$evidence/$phase-workflow-instances.json"
    cp "$scratch/workflow-task-instances.jsonl" "$evidence/$phase-workflow-task-instances.jsonl"
}

worker_marker_expected=$evidence/worker-pvc-marker-expected.txt
: > "$worker_marker_expected"

checkpoint_worker_pvc() {
    local phase=$1
    local actual="$scratch/$phase-worker-pvc-marker.txt"
    local helper="upgrade-worker-pvc-check-$phase"
    cat <<YAML | k apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: $helper
spec:
  securityContext:
    fsGroup: 1001
    fsGroupChangePolicy: OnRootMismatch
    seccompProfile:
      type: RuntimeDefault
  containers:
  - name: checker
    image: curlimages/curl:8.12.1@sha256:94e9e444bcba979c2ea12e27ae39bee4cd10bc7041a472c4727a558e213744e6
    command: ["sleep", "2147483647"]
    securityContext:
      allowPrivilegeEscalation: false
      capabilities:
        drop: ["ALL"]
      runAsGroup: 1001
      runAsNonRoot: true
      runAsUser: 1001
    volumeMounts:
    - name: worker-data
      mountPath: /data
  restartPolicy: Never
  volumes:
  - name: worker-data
    persistentVolumeClaim:
      claimName: worker-data-upgrade-worker-default-0
YAML
    wait_for "$phase worker PVC helper ready" pod_ready "$helper"

    if [[ -s "$worker_marker_expected" ]]; then
        k exec "pod/$helper" -c checker -- cat /data/framework-upgrade-pvc-marker > "$actual"
        cmp "$worker_marker_expected" "$actual"
    else
        k exec "pod/$helper" -c checker -- sh -ceu \
            'test ! -e /data/framework-upgrade-pvc-marker'
    fi

    printf '%s\n' "$phase" | k exec -i "pod/$helper" -c checker -- \
        sh -ceu 'umask 077; cat >> /data/framework-upgrade-pvc-marker; sync'
    printf '%s\n' "$phase" >> "$worker_marker_expected"
    k exec "pod/$helper" -c checker -- cat /data/framework-upgrade-pvc-marker \
        > "$evidence/$phase-worker-pvc-marker.txt"
    cmp "$worker_marker_expected" "$evidence/$phase-worker-pvc-marker.txt"

    k exec "pod/$helper" -c checker -- stat -c '%u:%g:%a' /data/framework-upgrade-pvc-marker \
        > "$evidence/$phase-worker-pvc-marker-owner.txt"
    test "$(<"$evidence/$phase-worker-pvc-marker-owner.txt")" = 1001:1001:600
    python3 - "$evidence/$phase-worker-pvc-marker.txt" > "$evidence/$phase-worker-pvc-marker-sha256.txt" <<'PY'
import hashlib
import pathlib
import sys

print(hashlib.sha256(pathlib.Path(sys.argv[1]).read_bytes()).hexdigest())
PY
    k get pod "$helper" -o yaml > "$evidence/$phase-worker-pvc-helper.yaml"
    k delete pod "$helper" --wait=true
}

snapshot() {
    local phase=$1
    k get statefulset,deployment,configmap,service,poddisruptionbudget,serviceaccount,role,rolebinding,job,persistentvolumeclaim \
        -o json > "$evidence/$phase-resources.json"
    k get dolphinschedulercluster "$cluster" -o yaml > "$evidence/$phase-cluster.yaml"
    k get persistentvolumeclaim -o json | python3 -c '
import json, sys
print(json.dumps(sorted(
    (item["metadata"]["name"], item["metadata"]["uid"], item["spec"].get("volumeName", ""))
    for item in json.load(sys.stdin)["items"]
)))
' > "$evidence/$phase-pvc-identities.json"
    k get job "$cluster" -o json | python3 -c '
import json, sys
job = json.load(sys.stdin)
print(json.dumps((job["metadata"]["name"], job["metadata"]["uid"])))
' > "$evidence/$phase-db-init-job-identity.json"
    k get statefulset/postgresql statefulset/upgrade-zk-server-default configmap/upgrade-znode \
        -o json | python3 -c '
import json, sys
print(json.dumps(sorted(
    (
        item["kind"],
        item["metadata"]["name"],
        item["metadata"]["uid"],
        item.get("data", {}),
    )
    for item in json.load(sys.stdin)["items"]
), sort_keys=True))
' > "$evidence/$phase-dependency-identities.json"
    k get secret postgresql-credentials -o json | python3 -c '
import hashlib, json, sys
secret = json.load(sys.stdin)
encoded = json.dumps(secret.get("data", {}), sort_keys=True, separators=(",", ":")).encode()
print(json.dumps({
    "name": secret["metadata"]["name"],
    "uid": secret["metadata"]["uid"],
    "resourceVersion": secret["metadata"]["resourceVersion"],
    "dataSha256": hashlib.sha256(encoded).hexdigest(),
}, sort_keys=True))
' > "$evidence/$phase-postgresql-credentials-identity.json"
}

render_operator "$UPGRADE_BASELINE_DIR" "$UPGRADE_BASELINE_IMAGE" \
    "$evidence/baseline-operator.yaml"
render_operator "$root" "$IMG" "$evidence/framework-operator.yaml"
kind load docker-image --name "$CHAINSAW_CLUSTER" "$UPGRADE_BASELINE_IMAGE" "$IMG"
deploy_operator "$evidence/baseline-operator.yaml"

kubectl create namespace "$namespace"
cat > "$evidence/input-dependencies.yaml" <<'YAML'
apiVersion: v1
kind: Secret
metadata:
  name: postgresql-credentials
type: Opaque
data:
  username: dGVzdA==
  password: dGVzdA==
---
apiVersion: v1
kind: Service
metadata:
  name: postgresql
spec:
  selector:
    app: postgresql
  ports:
  - name: postgresql
    port: 5432
    targetPort: 5432
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: postgresql
spec:
  replicas: 1
  selector:
    matchLabels:
      app: postgresql
  serviceName: postgresql
  template:
    metadata:
      labels:
        app: postgresql
    spec:
      containers:
      - name: postgresql
        image: postgres:13.4@sha256:1adb50e5c24f550a9e68457a2ce60e9e4103dfc43c3b36e98310168165b443a1
        env:
        - name: POSTGRES_USER
          value: test
        - name: POSTGRES_PASSWORD
          value: test
        - name: POSTGRES_DB
          value: dolphinscheduler
        - name: PGDATA
          value: /var/lib/postgresql/data/pgdata
        ports:
        - containerPort: 5432
        readinessProbe:
          exec:
            command: ["pg_isready", "-U", "test", "-d", "dolphinscheduler"]
          periodSeconds: 5
        volumeMounts:
        - name: postgres-data
          mountPath: /var/lib/postgresql/data
  volumeClaimTemplates:
  - metadata:
      name: postgres-data
    spec:
      accessModes: ["ReadWriteOnce"]
      resources:
        requests:
          storage: 1Gi
---
apiVersion: v1
kind: Pod
metadata:
  name: upgrade-api-client
spec:
  containers:
  - name: curl
    image: curlimages/curl:8.12.1@sha256:94e9e444bcba979c2ea12e27ae39bee4cd10bc7041a472c4727a558e213744e6
    command: ["sleep", "2147483647"]
  restartPolicy: Always
---
apiVersion: zookeeper.kubedoop.dev/v1alpha1
kind: ZookeeperCluster
metadata:
  name: upgrade-zk
spec:
  clusterConfig:
    listenerClass: cluster-internal
  servers:
    roleGroups:
      default:
        replicas: 1
---
apiVersion: zookeeper.kubedoop.dev/v1alpha1
kind: ZookeeperZnode
metadata:
  name: upgrade-znode
spec:
  clusterRef:
    name: upgrade-zk
YAML
k apply -f "$evidence/input-dependencies.yaml"
wait_for "PostgreSQL ready" statefulset_ready postgresql
wait_for "API client ready" pod_ready upgrade-api-client
wait_for "ZooKeeper ready" statefulset_ready upgrade-zk-server-default
wait_for "ZooKeeper discovery populated" znode_ready

cat > "$evidence/input-cluster.yaml" <<YAML
apiVersion: dolphinscheduler.kubedoop.dev/v1alpha1
kind: DolphinschedulerCluster
metadata:
  name: $cluster
spec:
  image:
    custom: "$UPGRADE_PRODUCT_IMAGE"
    productVersion: "$product_version"
  clusterConfig:
    zookeeperConfigMapName: upgrade-znode
    database:
      databaseType: postgresql
      connectionString: jdbc:postgresql://postgresql:5432/dolphinscheduler
      credentialsSecret: postgresql-credentials
  master:
    roleConfig:
      podDisruptionBudget:
        maxUnavailable: 1
    roleGroups:
      default:
        replicas: 1
        config:
          resources:
            cpu:
              min: 500m
              max: "1"
            memory:
              limit: 2Gi
  worker:
    roleConfig:
      podDisruptionBudget:
        maxUnavailable: 1
    roleGroups:
      default:
        replicas: 1
        envOverrides:
          # The product container already runs as the matching kubedoop tenant;
          # it intentionally does not ship sudo.
          SUDO_ENABLE: "false"
  api:
    roleConfig:
      podDisruptionBudget:
        maxUnavailable: 1
    roleGroups:
      default:
        replicas: 1
  alerter:
    roleConfig:
      podDisruptionBudget:
        maxUnavailable: 1
    roleGroups:
      default:
        replicas: 1
YAML
k apply -f "$evidence/input-cluster.yaml"
wait_for "baseline database initialization" job_complete "$cluster"
wait_product_ready before
wait_for "baseline DolphinScheduler API ready" api_ready
create_acceptance_workflow
run_workflow before 1
checkpoint_worker_pvc before
snapshot before

echo "Upgrade: replace only incompatible DolphinScheduler workloads; retain DB, ZooKeeper and PVCs"
replace_product_workloads
delete_namespaced_rbac dolphinscheduler
deploy_operator "$evidence/framework-operator.yaml"
wait_for "framework database initialization" job_complete "$cluster"
wait_product_ready after
wait_for "framework DolphinScheduler API ready" api_ready
verify_acceptance_workflow
run_workflow after 2
checkpoint_worker_pvc after
snapshot after
cmp "$evidence/before-pvc-identities.json" "$evidence/after-pvc-identities.json"

echo "Rollback: rebuild the Gen2b resource shape without downgrading the expanded CRD"
replace_product_workloads
delete_namespaced_rbac dolphinschedulercluster-upgrade
# The Gen3 DB Job is intentionally incompatible with the Gen2b Job (secure
# SecretKeyRefs, target identity and no xtrace).  A completed Job has immutable
# pod fields, so recreate this one-shot workload while retaining PostgreSQL and
# its PVC.  The third successful workflow below proves the schema/data survived.
k delete job "$cluster" --wait=true
python3 - "$evidence/baseline-operator.yaml" "$scratch/rollback-operator.yaml" <<'PY'
import pathlib
import re
import sys

documents = re.split(r"^---\s*$", pathlib.Path(sys.argv[1]).read_text(), flags=re.MULTILINE)
documents = [
    document for document in documents
    if not re.search(r"^kind:\s*CustomResourceDefinition\s*$", document, flags=re.MULTILINE)
]
pathlib.Path(sys.argv[2]).write_text("\n---\n".join(documents))
PY
deploy_operator "$scratch/rollback-operator.yaml"
wait_for "rollback database initialization" job_complete "$cluster"
wait_product_ready rollback
wait_for "rollback DolphinScheduler API ready" api_ready
verify_acceptance_workflow
run_workflow rollback 3
checkpoint_worker_pvc rollback
snapshot rollback
cmp "$evidence/before-pvc-identities.json" "$evidence/rollback-pvc-identities.json"
PRODUCT_VERSION="$product_version" python3 "$root/hack/compare-upgrade-resources.py" "$evidence"

kubectl delete namespace "$namespace" --wait=true --timeout=300s
deploy_operator "$evidence/framework-operator.yaml"
printf '%s\n' \
    "PASS: DolphinScheduler $product_version baseline -> framework -> baseline; 3 Shell tasks and worker/DB PVC data preserved" \
    | tee "$evidence/result.txt"
