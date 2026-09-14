"""Validate and record the Gen2b -> Gen3 -> Gen2b resource contract.

The upgrade is intentionally destructive at the workload layer.  This checker
therefore verifies the documented shape on each side of the migration, proves
that no Deployment/StatefulSet pair serves the same role at once, and requires
the rollback to reproduce the normalized baseline exactly.
"""

import copy
import difflib
import hashlib
import json
import os
import pathlib
import sys
from collections import Counter


CLUSTER = "upgrade"
ROLES = ("master", "worker", "api", "alert")
NAMESPACE = "dolphinscheduler-framework-upgrade"
PRODUCT_NAME = "dolphinscheduler"
PRODUCT_VERSION = os.environ.get("PRODUCT_VERSION", "3.2.2")
LABEL_DOMAIN = "dolphinscheduler.kubedoop.dev"
DB_INIT_IMAGE_ANNOTATION = f"{LABEL_DOMAIN}/db-init-image"
DB_INIT_TARGET_ANNOTATION = f"{LABEL_DOMAIN}/db-init-target-hash"
DB_INIT_CREDENTIALS_ANNOTATION = f"{LABEL_DOMAIN}/db-init-credentials-version-hash"
RUNTIME_DEPENDENCY_ANNOTATION = f"{LABEL_DOMAIN}/runtime-dependency-hash"
JOB_UID_TOKEN = "<job-controller-uid>"
MAIN_CONTAINERS = {
    "master": "master-server",
    "worker": "worker-server",
    "api": "api-server",
    "alert": "alerter-server",
}
METRICS_PORTS = {"master": 5679, "worker": 1235, "api": 12345, "alert": 50053}
NORMALIZED_KINDS = {
    "ConfigMap",
    "Deployment",
    "Job",
    "PodDisruptionBudget",
    "Role",
    "RoleBinding",
    "Service",
    "ServiceAccount",
    "StatefulSet",
}


def load_resources(root: pathlib.Path, phase: str) -> dict[tuple[str, str], dict]:
    items = json.loads((root / f"{phase}-resources.json").read_text())["items"]
    return {(item["kind"], item["metadata"]["name"]): item for item in items}


def has(items: dict[tuple[str, str], dict], kind: str, name: str) -> bool:
    return (kind, name) in items


def require(items: dict[tuple[str, str], dict], kind: str, name: str) -> dict:
    try:
        return items[kind, name]
    except KeyError as error:
        raise AssertionError(f"missing {kind}/{name}") from error


def legacy_cluster_labels() -> dict[str, str]:
    return {
        "app.kubernetes.io/instance": CLUSTER,
        "app.kubernetes.io/managed-by": LABEL_DOMAIN,
        "app.kubernetes.io/name": "dolphinschedulercluster",
    }


def legacy_role_labels(role: str, group: bool = True) -> dict[str, str]:
    labels = legacy_cluster_labels()
    labels["app.kubernetes.io/component"] = role
    if group:
        labels["app.kubernetes.io/role-group"] = "default"
    return labels


def framework_role_labels(role: str, group: bool = True) -> dict[str, str]:
    labels = {
        "app.kubernetes.io/component": role,
        "app.kubernetes.io/instance": CLUSTER,
        "app.kubernetes.io/managed-by": "operator-go",
        "app.kubernetes.io/name": PRODUCT_NAME,
        "app.kubernetes.io/version": PRODUCT_VERSION,
        f"{LABEL_DOMAIN}/cluster": CLUSTER,
        f"{LABEL_DOMAIN}/role": role,
    }
    if group:
        labels.update(
            {
                "app.kubernetes.io/role-group": "default",
                f"{LABEL_DOMAIN}/role-group": "default",
                f"{CLUSTER}-default": "true",
            }
        )
    return labels


def framework_selector(role: str, group: bool = True) -> dict[str, str]:
    selector = {
        f"{LABEL_DOMAIN}/cluster": CLUSTER,
        f"{LABEL_DOMAIN}/role": role,
    }
    if group:
        selector[f"{LABEL_DOMAIN}/role-group"] = "default"
    return selector


def object_annotations(item: dict) -> dict:
    return item["metadata"].get("annotations", {})


def assert_labels(item: dict, expected: dict[str, str], description: str) -> None:
    actual = item["metadata"].get("labels", {})
    assert actual == expected, (description, "labels drift", actual, expected)


def assert_selector(selector: dict, expected: dict[str, str], description: str) -> None:
    assert selector == {"matchLabels": expected}, (
        description,
        "selector drift",
        selector,
        expected,
    )


def assert_workload_shape(items: dict[tuple[str, str], dict], phase: str) -> None:
    framework = phase == "after"
    for role in ROLES:
        name = f"{CLUSTER}-{role}-default"
        expect_statefulset = framework or role in {"master", "worker"}
        expect_deployment = not framework and role in {"api", "alert"}
        assert has(items, "StatefulSet", name) == expect_statefulset, (
            phase,
            f"unexpected StatefulSet state for {name}",
        )
        assert has(items, "Deployment", name) == expect_deployment, (
            phase,
            f"unexpected Deployment state for {name}",
        )
        assert not (has(items, "StatefulSet", name) and has(items, "Deployment", name)), (
            phase,
            f"double workload for {name}",
        )


def assert_service_shape(items: dict[tuple[str, str], dict], phase: str) -> None:
    framework = phase == "after"
    for role in ROLES:
        name = f"{CLUSTER}-{role}-default"
        client = require(items, "Service", name)
        expected_client_headless = not framework and role in {"master", "worker"}
        assert (client["spec"].get("clusterIP") == "None") == expected_client_headless, (
            phase,
            f"wrong client Service type for {name}",
        )

        headless_name = name + "-headless"
        if framework:
            headless = require(items, "Service", headless_name)
            assert headless["spec"].get("clusterIP") == "None", (
                phase,
                f"{headless_name} is not headless",
            )
        else:
            assert not has(items, "Service", headless_name), (
                phase,
                f"framework-only Service {headless_name} survived",
            )


def volume_claim(statefulset: dict, name: str) -> dict:
    for claim in statefulset["spec"].get("volumeClaimTemplates", []):
        if claim["metadata"]["name"] == name:
            return claim
    raise AssertionError(
        f"missing volumeClaimTemplate {name} on StatefulSet/{statefulset['metadata']['name']}"
    )


def assert_phase_contracts(phases: dict[str, dict[tuple[str, str], dict]]) -> None:
    for phase, items in phases.items():
        assert_workload_shape(items, phase)
        assert_service_shape(items, phase)
        for role in ROLES:
            require(items, "PodDisruptionBudget", f"{CLUSTER}-{role}")
            require(items, "Service", f"{CLUSTER}-{role}-default-metrics")
        require(items, "Job", CLUSTER)

    before = phases["before"]
    after = phases["after"]
    rollback = phases["rollback"]

    require(before, "ServiceAccount", "dolphinscheduler")
    require(before, "Role", "dolphinscheduler")
    require(before, "RoleBinding", "dolphinscheduler")
    assert not has(before, "ServiceAccount", "dolphinschedulercluster-upgrade")
    require(after, "ServiceAccount", "dolphinschedulercluster-upgrade")
    require(after, "Role", "dolphinschedulercluster-upgrade")
    require(after, "RoleBinding", "dolphinschedulercluster-upgrade")
    assert not has(after, "ServiceAccount", "dolphinscheduler")
    require(rollback, "ServiceAccount", "dolphinscheduler")
    require(rollback, "Role", "dolphinscheduler")
    require(rollback, "RoleBinding", "dolphinscheduler")
    assert not has(rollback, "ServiceAccount", "dolphinschedulercluster-upgrade")

    for phase, items in phases.items():
        worker = require(items, "StatefulSet", f"{CLUSTER}-worker-default")
        claim = volume_claim(worker, "worker-data")
        assert claim["spec"]["resources"]["requests"]["storage"] == "2Gi", (
            phase,
            "worker-data capacity changed",
        )

    before_claim = volume_claim(
        require(before, "StatefulSet", f"{CLUSTER}-worker-default"), "worker-data"
    )
    for phase in ("after", "rollback"):
        current_claim = volume_claim(
            require(phases[phase], "StatefulSet", f"{CLUSTER}-worker-default"),
            "worker-data",
        )
        assert current_claim["spec"] == before_claim["spec"], (
            phase,
            "worker-data claim specification changed",
        )


def assert_pvc_identity(root: pathlib.Path) -> None:
    identities = {
        phase: json.loads((root / f"{phase}-pvc-identities.json").read_text())
        for phase in ("before", "after", "rollback")
    }
    assert identities["before"] == identities["after"] == identities["rollback"], (
        "PVC UID/PV binding changed",
        identities,
    )
    names = {entry[0] for entry in identities["before"]}
    assert "worker-data-upgrade-worker-default-0" in names, "worker PVC was not captured"
    assert "postgres-data-postgresql-0" in names, "PostgreSQL PVC was not captured"


def assert_db_init_job_identity(root: pathlib.Path) -> None:
    identities = {
        phase: json.loads((root / f"{phase}-db-init-job-identity.json").read_text())
        for phase in ("before", "after", "rollback")
    }
    assert all(identity[0] == CLUSTER and identity[1] for identity in identities.values())
    assert len({identity[1] for identity in identities.values()}) == 3, (
        "each documented DB-init compatibility transition must rebuild the Job",
        identities,
    )


def assert_dependency_identity(root: pathlib.Path) -> None:
    identities = {
        phase: json.loads((root / f"{phase}-dependency-identities.json").read_text())
        for phase in ("before", "after", "rollback")
    }
    assert identities["before"] == identities["after"] == identities["rollback"], (
        "PostgreSQL/ZooKeeper identity or Znode discovery data changed",
        identities,
    )

    secret_identities = {
        phase: json.loads(
            (root / f"{phase}-postgresql-credentials-identity.json").read_text()
        )
        for phase in ("before", "after", "rollback")
    }
    assert (
        secret_identities["before"]
        == secret_identities["after"]
        == secret_identities["rollback"]
    ), ("PostgreSQL credential Secret UID or data checksum changed", secret_identities)


def expected_runtime_dependency_fingerprint(
    root: pathlib.Path, after: dict[tuple[str, str], dict]
) -> str:
    secret = json.loads(
        (root / "after-postgresql-credentials-identity.json").read_text()
    )
    dependencies = [
        (
            f"ConfigMap/{NAMESPACE}/{name}="
            f'{require(after, "ConfigMap", name)["metadata"]["resourceVersion"]}'
        )
        for name in (f"{CLUSTER}-envs", "upgrade-znode")
    ]
    dependencies.append(
        f"Secret/{NAMESPACE}/postgresql-credentials={secret['resourceVersion']}"
    )
    return hashlib.sha256("\n".join(sorted(dependencies)).encode()).hexdigest()


def belongs_to_dolphinscheduler(item: dict) -> bool:
    if item["kind"] not in NORMALIZED_KINDS:
        return False
    if item["kind"] in {"Role", "RoleBinding", "ServiceAccount"} and item["metadata"]["name"] in {
        "dolphinscheduler",
        "dolphinschedulercluster-upgrade",
    }:
        return True
    return any(
        owner.get("apiVersion", "").startswith("dolphinscheduler.kubedoop.dev/")
        and owner.get("kind") == "DolphinschedulerCluster"
        and owner.get("name") == CLUSTER
        for owner in item["metadata"].get("ownerReferences", [])
    )


def expected_inventory(phase: str) -> set[tuple[str, str]]:
    framework = phase == "after"
    rbac_name = "dolphinschedulercluster-upgrade" if framework else "dolphinscheduler"
    expected = {
        ("ConfigMap", f"{CLUSTER}-envs"),
        ("Job", CLUSTER),
        ("Role", rbac_name),
        ("RoleBinding", rbac_name),
        ("ServiceAccount", rbac_name),
    }
    for role in ROLES:
        name = f"{CLUSTER}-{role}-default"
        expected.update(
            {
                ("ConfigMap", name),
                ("PodDisruptionBudget", f"{CLUSTER}-{role}"),
                ("Service", name),
                ("Service", name + "-metrics"),
            }
        )
        if framework:
            expected.add(("StatefulSet", name))
            expected.add(("Service", name + "-headless"))
        elif role in {"master", "worker"}:
            expected.add(("StatefulSet", name))
        else:
            expected.add(("Deployment", name))
    return expected


def assert_exact_inventory(phases: dict[str, dict[tuple[str, str], dict]]) -> None:
    for phase, items in phases.items():
        actual = {
            key for key, item in items.items() if belongs_to_dolphinscheduler(item)
        }
        expected = expected_inventory(phase)
        assert actual == expected, (
            phase,
            "DolphinScheduler resource inventory drift",
            "missing",
            sorted(expected - actual),
            "unexpected",
            sorted(actual - expected),
        )


def canonical_metadata(item: dict, name: str | None = None) -> dict:
    """Keep all metadata except server state and the separately checked labels."""

    metadata = copy.deepcopy(item["metadata"])
    for key in (
        "creationTimestamp",
        "generation",
        "labels",
        "managedFields",
        "resourceVersion",
        "uid",
    ):
        metadata.pop(key, None)
    if name is not None:
        metadata["name"] = name
    annotations = metadata.get("annotations", {})
    annotations.pop("banzaicloud.com/last-applied", None)
    annotations.pop("deployment.kubernetes.io/revision", None)
    annotations.pop("kubectl.kubernetes.io/last-applied-configuration", None)
    if not annotations:
        metadata.pop("annotations", None)
    for owner in metadata.get("ownerReferences", []):
        owner.pop("uid", None)
    return metadata


def main_container(item: dict, role: str) -> dict:
    containers = item["spec"]["template"]["spec"].get("containers", [])
    assert len(containers) == 1, (role, "unexpected sidecar/main-container count", containers)
    container = containers[0]
    assert container["name"] == MAIN_CONTAINERS[role], (
        role,
        "main container name drift",
        container["name"],
    )
    return container


def expected_probe(role: str, endpoint: str, startup: bool = False) -> dict:
    probe = {
        "exec": {
            "command": [
                "curl",
                f"http://localhost:{METRICS_PORTS[role]}/actuator/health/{endpoint}",
            ]
        },
        "failureThreshold": 60 if startup else 3,
        "periodSeconds": 10 if startup else 30,
        "successThreshold": 1,
        "timeoutSeconds": 5,
    }
    if not startup:
        probe["initialDelaySeconds"] = 30
    return probe


def encoded_counter(items: list[dict]) -> Counter:
    return Counter(json.dumps(item, sort_keys=True, separators=(",", ":")) for item in items)


def assert_environment_contract(old: dict, new: dict, role: str) -> None:
    old_env = old.get("env", [])
    new_env = new.get("env", [])
    extras = [
        {"name": "TZ", "value": "Asia/Shanghai"},
        {"name": "SPRING_JACKSON_TIME_ZONE", "value": "Asia/Shanghai"},
        {
            "name": "SPRING_DATASOURCE_USERNAME",
            "valueFrom": {
                "secretKeyRef": {"key": "username", "name": "postgresql-credentials"}
            },
        },
        {
            "name": "SPRING_DATASOURCE_PASSWORD",
            "valueFrom": {
                "secretKeyRef": {"key": "password", "name": "postgresql-credentials"}
            },
        },
    ]
    if role == "worker":
        extras.append({"name": "SUDO_ENABLE", "value": "false"})

    old_names = {entry["name"] for entry in old_env}
    assert not (old_names & {entry["name"] for entry in extras}), (
        role,
        "Gen3-only env already appeared directly in the baseline",
    )
    assert encoded_counter(new_env) == encoded_counter(old_env) + encoded_counter(extras), (
        role,
        "container environment drift outside credential/time-zone migration",
    )


def assert_command_contract(old: dict, new: dict, role: str) -> None:
    assert old.get("command") == ["/bin/bash", "-x", "-euo", "pipefail", "-c"], (
        role,
        "legacy shell command drift",
    )
    old_args = old.get("args", [])
    assert len(old_args) == 1, (role, "legacy start script must be the sole arg")
    legacy_script = old_args[0]
    before_marker = "rm -f /kubedoop/log/_vector/shutdown"
    after_marker = (
        "mkdir -p /kubedoop/log/_vector/ && "
        "touch /kubedoop/log/_vector/shutdown"
    )
    old_start = f"{role}-server/bin/start.sh &"
    new_start = f'{role}-server/bin/start.sh "$@" &'
    assert legacy_script.count(before_marker) == 1, (role, "legacy pre-start marker drift")
    assert legacy_script.count(after_marker) == 1, (role, "legacy shutdown marker drift")
    assert legacy_script.count(old_start) == 1, (role, "legacy role start command drift")
    expected_script = "\n".join(
        new_start if line == old_start else line
        for line in legacy_script.splitlines()
        if line not in {before_marker, after_marker}
    )
    assert new.get("command") == [
        "/bin/bash",
        "-euo",
        "pipefail",
        "-c",
        expected_script,
        "dolphinscheduler",
    ], (role, "Gen3 command differs beyond xtrace/Vector-marker/argv migration")
    assert new.get("args", []) == [], (role, "fixture has no cliOverrides; Args must be empty")


def assert_volume_contract(old_spec: dict, new_spec: dict, role: str) -> None:
    name = f"{CLUSTER}-{role}-default"
    old_volumes = {volume["name"]: volume for volume in old_spec.get("volumes", [])}
    new_volumes = {volume["name"]: volume for volume in new_spec.get("volumes", [])}
    assert set(old_volumes) == {"common-config", "logback", "log"}, (
        role,
        "legacy volume inventory drift",
        sorted(old_volumes),
    )
    assert set(new_volumes) == {"config"}, (
        role,
        "Gen3 volume inventory drift",
        sorted(new_volumes),
    )

    old_config = copy.deepcopy(old_volumes["common-config"])
    new_config = copy.deepcopy(new_volumes["config"])
    old_config["name"] = "config"
    assert new_config == old_config, (role, "role ConfigMap volume source drift")
    assert old_config.get("configMap", {}).get("name") == name

    log_volume = old_volumes["log"]
    assert set(log_volume) == {"name", "emptyDir"}, (role, "legacy log volume source drift")
    assert log_volume["emptyDir"] == {"sizeLimit": "30Mi"}, (
        role,
        "legacy log volume size drift",
    )
    logback = old_volumes["logback"]
    assert set(logback) == {"name", "configMap"}, (role, "legacy logback volume drift")
    logback_source = logback["configMap"]
    assert logback_source.get("name") == name
    assert logback_source.get("items") == [
        {"key": "logback-spring.xml", "path": "logback-spring.xml"}
    ]
    assert set(logback_source) <= {"name", "items", "defaultMode", "optional"}
    assert logback_source.get("optional") in (None, False)


def assert_mount_contract(old: dict, new: dict, role: str) -> None:
    server = f"{role}-server"
    common_path = f"/kubedoop/dolphinscheduler/{server}/conf/common.properties"
    logback_path = f"/kubedoop/dolphinscheduler/{server}/conf/logback-spring.xml"
    old_mounts = {mount["mountPath"]: mount for mount in old.get("volumeMounts", [])}
    new_mounts = {mount["mountPath"]: mount for mount in new.get("volumeMounts", [])}
    expected_old_paths = {common_path, logback_path}
    expected_new_paths = expected_old_paths | {"/kubedoop/mount/config/"}
    if role == "worker":
        expected_new_paths.add("/tmp/dolphinscheduler")
    assert set(old_mounts) == expected_old_paths, (role, "legacy mount inventory drift")
    assert set(new_mounts) == expected_new_paths, (role, "Gen3 mount inventory drift")

    for path in (common_path, logback_path):
        canonical_new = copy.deepcopy(new_mounts[path])
        canonical_new["name"] = old_mounts[path]["name"]
        assert canonical_new == old_mounts[path], (role, path, "subPath mount drift")
    assert old_mounts[common_path]["name"] == "common-config"
    assert old_mounts[logback_path]["name"] == "logback"
    assert new_mounts[common_path]["name"] == "config"
    assert new_mounts[logback_path]["name"] == "config"
    assert new_mounts["/kubedoop/mount/config/"] == {
        "mountPath": "/kubedoop/mount/config/",
        "name": "config",
        "readOnly": True,
    }
    if role == "worker":
        assert new_mounts["/tmp/dolphinscheduler"] == {
            "mountPath": "/tmp/dolphinscheduler",
            "name": "worker-data",
        }


def assert_container_contract(old_item: dict, new_item: dict, role: str) -> None:
    old = copy.deepcopy(main_container(old_item, role))
    new = copy.deepcopy(main_container(new_item, role))

    assert old.get("readinessProbe") == expected_probe(role, "readiness")
    assert old.get("livenessProbe") == expected_probe(role, "liveness")
    assert new.get("readinessProbe") == old.get("readinessProbe")
    assert new.get("livenessProbe") == old.get("livenessProbe")
    assert "startupProbe" not in old
    assert new.get("startupProbe") == expected_probe(role, "readiness", startup=True)
    for container in (old, new):
        container.pop("readinessProbe", None)
        container.pop("livenessProbe", None)
    new.pop("startupProbe")

    assert old.get("securityContext") in (None, {})
    assert new.get("securityContext") == {
        "allowPrivilegeEscalation": False,
        "capabilities": {"drop": ["ALL"]},
        "runAsGroup": 0,
        "runAsNonRoot": True,
        "runAsUser": 1001,
        "seccompProfile": {"type": "RuntimeDefault"},
    }
    old.pop("securityContext", None)
    new.pop("securityContext")

    assert_environment_contract(old, new, role)
    old.pop("env", None)
    new.pop("env", None)
    assert_command_contract(old, new, role)
    for container in (old, new):
        container.pop("command", None)
        container.pop("args", None)
    assert_mount_contract(old, new, role)
    for container in (old, new):
        container.pop("volumeMounts", None)

    # Everything not named above (image, pull policy, ports, resources,
    # envFrom, termination settings, and future fields) must be identical.
    assert new == old, (role, "unclassified main-container drift", old, new)


def assert_pod_template_contract(
    old_item: dict, new_item: dict, role: str, dependency_fingerprint: str
) -> None:
    old = copy.deepcopy(old_item["spec"]["template"])
    new = copy.deepcopy(new_item["spec"]["template"])
    assert old["metadata"].get("labels", {}) == legacy_role_labels(role)
    assert new["metadata"].get("labels", {}) == framework_role_labels(role)
    old["metadata"].pop("labels", None)
    new["metadata"].pop("labels", None)
    old_annotations = old["metadata"].get("annotations", {})
    new_annotations = new["metadata"].get("annotations", {})
    assert RUNTIME_DEPENDENCY_ANNOTATION not in old_annotations
    assert new_annotations == old_annotations | {
        RUNTIME_DEPENDENCY_ANNOTATION: dependency_fingerprint
    }, (role, "runtime dependency fingerprint annotation drift")
    new_annotations.pop(RUNTIME_DEPENDENCY_ANNOTATION)
    if not new_annotations:
        new["metadata"].pop("annotations", None)
    assert new["metadata"] == old["metadata"], (role, "pod-template metadata drift")

    old_spec = old["spec"]
    new_spec = new["spec"]
    assert old_spec.get("securityContext") in (None, {})
    assert new_spec.get("securityContext") == {
        "fsGroup": 1001,
        "fsGroupChangePolicy": "OnRootMismatch",
        "runAsGroup": 0,
        "runAsNonRoot": True,
        "runAsUser": 1001,
        "seccompProfile": {"type": "RuntimeDefault"},
    }
    old_spec.pop("securityContext", None)
    new_spec.pop("securityContext")

    assert old_spec.get("enableServiceLinks") in (None, True)
    assert new_spec.get("enableServiceLinks") is False
    old_spec.pop("enableServiceLinks", None)
    new_spec.pop("enableServiceLinks")
    for field in ("serviceAccount", "serviceAccountName"):
        if field in old_spec or field in new_spec:
            assert old_spec.get(field) == "dolphinscheduler", (role, field, "legacy SA drift")
            assert new_spec.get(field) == "dolphinschedulercluster-upgrade", (
                role,
                field,
                "Gen3 SA drift",
            )
            old_spec.pop(field, None)
            new_spec.pop(field, None)

    assert_container_contract(old_item, new_item, role)
    old_spec.pop("containers")
    new_spec.pop("containers")
    assert_volume_contract(old_spec, new_spec, role)
    old_spec.pop("volumes", None)
    new_spec.pop("volumes", None)
    assert new_spec == old_spec, (role, "unclassified pod-spec drift", old_spec, new_spec)


def service_spec_without_allocations(item: dict) -> dict:
    spec = copy.deepcopy(item["spec"])
    for field in (
        "clusterIP",
        "clusterIPs",
        "healthCheckNodePort",
        "ipFamilies",
        "ipFamilyPolicy",
    ):
        spec.pop(field, None)
    for port in spec.get("ports", []):
        port.pop("nodePort", None)
    return spec


def assert_service_contract(before: dict, after: dict, role: str) -> None:
    name = f"{CLUSTER}-{role}-default"
    old = require(before, "Service", name)
    new = require(after, "Service", name)
    assert_labels(old, legacy_role_labels(role), name)
    assert_labels(new, framework_role_labels(role), name)
    assert canonical_metadata(new) == canonical_metadata(old), (name, "client metadata drift")
    assert old["spec"].get("selector") == legacy_role_labels(role)
    assert new["spec"].get("selector") == framework_selector(role)
    old_spec = service_spec_without_allocations(old)
    new_spec = service_spec_without_allocations(new)
    old_spec.pop("selector")
    new_spec.pop("selector")
    assert new_spec == old_spec, (name, "client Service drift outside topology/selector")
    assert new["spec"].get("clusterIP") != "None", (name, "client Service stayed headless")

    headless = require(after, "Service", name + "-headless")
    assert_labels(headless, framework_role_labels(role), name + "-headless")
    assert canonical_metadata(headless) == canonical_metadata(new, name=name + "-headless")
    assert headless["spec"].get("clusterIP") == "None"
    assert headless["spec"].get("selector") == framework_selector(role)
    headless_spec = service_spec_without_allocations(headless)
    headless_spec.pop("selector")
    assert headless_spec == new_spec, (name, "headless/client Service contract diverged")

    metrics_name = name + "-metrics"
    old_metrics = require(before, "Service", metrics_name)
    new_metrics = require(after, "Service", metrics_name)
    metrics_labels = legacy_role_labels(role) | {"prometheus.io/scrape": "true"}
    assert_labels(old_metrics, metrics_labels, metrics_name)
    assert_labels(new_metrics, metrics_labels, metrics_name)
    expected_metrics_annotations = {
        "prometheus.io/path": "/actuator/prometheus",
        "prometheus.io/port": str(METRICS_PORTS[role]),
        "prometheus.io/scheme": "http",
        "prometheus.io/scrape": "true",
    }
    assert canonical_metadata(old_metrics).get("annotations", {}) == (
        expected_metrics_annotations
    ), (role, "legacy metrics discovery annotations drift")
    assert canonical_metadata(new_metrics).get("annotations", {}) == (
        expected_metrics_annotations
    ), (role, "Gen3 metrics discovery annotations drift")
    assert canonical_metadata(new_metrics) == canonical_metadata(old_metrics)
    assert old_metrics["spec"].get("clusterIP") == "None"
    assert new_metrics["spec"].get("clusterIP") == "None"
    assert old_metrics["spec"].get("selector") == legacy_role_labels(role)
    assert new_metrics["spec"].get("selector") == framework_selector(role)
    old_metrics_spec = service_spec_without_allocations(old_metrics)
    new_metrics_spec = service_spec_without_allocations(new_metrics)
    old_metrics_spec.pop("selector")
    new_metrics_spec.pop("selector")
    assert len(old_metrics_spec["ports"]) == len(new_metrics_spec["ports"]) == 1
    old_port = old_metrics_spec["ports"][0]
    new_port = new_metrics_spec["ports"][0]
    assert old_port.get("targetPort") == "metrics", (role, "legacy metrics target drift")
    assert new_port.get("port") == METRICS_PORTS[role]
    assert new_port.get("targetPort") == METRICS_PORTS[role], (
        role,
        "Gen3 metrics target must be the real numeric port",
    )
    old_port.pop("targetPort")
    new_port.pop("targetPort")
    assert new_metrics_spec == old_metrics_spec, (metrics_name, "unclassified metrics drift")


def assert_statefulset_policy(new: dict, role: str) -> None:
    spec = new["spec"]
    assert spec.get("podManagementPolicy") == "Parallel", (role, "Gen3 policy is not Parallel")
    strategy = spec.get("updateStrategy", {})
    assert strategy.get("type") == "RollingUpdate"
    assert set(strategy) <= {"type", "rollingUpdate"}
    assert strategy.get("rollingUpdate") in (None, {"partition": 0}), (
        role,
        "Gen3 rollout partition/maxUnavailable drift",
        strategy,
    )
    assert spec.get("revisionHistoryLimit") == 10
    assert spec.get("minReadySeconds", 0) == 0
    retention = spec.get("persistentVolumeClaimRetentionPolicy")
    if retention is not None:
        assert retention == {"whenDeleted": "Retain", "whenScaled": "Retain"}, (
            role,
            "PVC retention must never permit deletion",
            retention,
        )
    ordinals = spec.get("ordinals")
    if ordinals is not None:
        assert ordinals == {"start": 0}


def assert_rbac_contract(before: dict, after: dict) -> None:
    old_name = "dolphinscheduler"
    new_name = "dolphinschedulercluster-upgrade"
    expected_rules = [
        {"apiGroups": [""], "resources": ["configmaps"], "verbs": ["get", "list", "watch"]}
    ]
    new_labels = {
        "app.kubernetes.io/instance": CLUSTER,
        "app.kubernetes.io/managed-by": "operator-go",
    }

    old_role = require(before, "Role", old_name)
    new_role = require(after, "Role", new_name)
    assert_labels(old_role, legacy_cluster_labels(), "legacy workload Role")
    assert_labels(new_role, new_labels, "Gen3 workload Role")
    assert old_role.get("rules") == new_role.get("rules") == expected_rules
    assert canonical_metadata(new_role, old_name) == canonical_metadata(old_role)

    old_sa = require(before, "ServiceAccount", old_name)
    new_sa = require(after, "ServiceAccount", new_name)
    assert_labels(old_sa, legacy_cluster_labels(), "legacy workload ServiceAccount")
    assert_labels(new_sa, new_labels, "Gen3 workload ServiceAccount")
    assert canonical_metadata(new_sa, old_name) == canonical_metadata(old_sa)
    old_sa_spec = {key: value for key, value in old_sa.items() if key not in {"apiVersion", "kind", "metadata"}}
    new_sa_spec = {key: value for key, value in new_sa.items() if key not in {"apiVersion", "kind", "metadata"}}
    assert new_sa_spec == old_sa_spec, "workload ServiceAccount contract drift"

    old_binding = require(before, "RoleBinding", old_name)
    new_binding = require(after, "RoleBinding", new_name)
    assert_labels(old_binding, legacy_cluster_labels(), "legacy workload RoleBinding")
    assert_labels(new_binding, new_labels, "Gen3 workload RoleBinding")
    expected_old_subjects = [
        {"kind": "ServiceAccount", "name": old_name, "namespace": NAMESPACE}
    ]
    expected_new_subjects = [
        {"kind": "ServiceAccount", "name": new_name, "namespace": NAMESPACE}
    ]
    assert old_binding.get("subjects") == expected_old_subjects
    assert new_binding.get("subjects") == expected_new_subjects
    assert old_binding.get("roleRef") == {
        "apiGroup": "rbac.authorization.k8s.io",
        "kind": "Role",
        "name": old_name,
    }
    assert new_binding.get("roleRef") == {
        "apiGroup": "rbac.authorization.k8s.io",
        "kind": "Role",
        "name": new_name,
    }
    assert canonical_metadata(new_binding, old_name) == canonical_metadata(old_binding)


def canonical_job_controller_identity(job: dict) -> dict:
    """Validate and tokenize only the API server's per-Job UID labels."""

    value = copy.deepcopy(job)
    uid = value["metadata"].get("uid")
    name = value["metadata"]["name"]
    assert uid, (name, "Job UID is empty")
    controller_key = "batch.kubernetes.io/controller-uid"
    template_auto_labels = {
        controller_key: uid,
        "controller-uid": uid,
        "batch.kubernetes.io/job-name": name,
        "job-name": name,
    }
    selector = value["spec"].get("selector")
    assert selector == {"matchLabels": {controller_key: uid}}, (
        name,
        "unexpected API-generated Job selector",
        selector,
    )
    template_labels = value["spec"]["template"]["metadata"].get("labels", {})
    base_labels = legacy_cluster_labels()
    assert template_labels == base_labels | template_auto_labels, (
        name,
        "unexpected API-generated Job template labels",
        template_labels,
    )

    value["spec"]["selector"]["matchLabels"][controller_key] = JOB_UID_TOKEN
    value["spec"]["template"]["metadata"]["labels"].update(
        {
            controller_key: JOB_UID_TOKEN,
            "controller-uid": JOB_UID_TOKEN,
        }
    )
    return value


def assert_db_job_contract(
    root: pathlib.Path, before: dict, after: dict, rollback: dict
) -> None:
    old = canonical_job_controller_identity(require(before, "Job", CLUSTER))
    new = canonical_job_controller_identity(require(after, "Job", CLUSTER))
    rolled_back = canonical_job_controller_identity(require(rollback, "Job", CLUSTER))
    assert_labels(old, legacy_cluster_labels(), "legacy database-init Job")
    assert_labels(new, legacy_cluster_labels(), "Gen3 database-init Job")
    assert_labels(rolled_back, legacy_cluster_labels(), "rollback database-init Job")
    for phase, job in (("before", old), ("after", new), ("rollback", rolled_back)):
        assert job.get("status", {}).get("succeeded", 0) >= 1, (
            phase,
            "database initialization Job did not succeed",
        )
        conditions = {
            condition["type"]: condition["status"]
            for condition in job.get("status", {}).get("conditions", [])
        }
        assert conditions.get("Complete") == "True", (phase, "DB Job is not Complete")
        owners = job["metadata"].get("ownerReferences", [])
        assert len(owners) == 1
        assert owners[0].get("apiVersion", "").startswith(f"{LABEL_DOMAIN}/")
        assert owners[0].get("kind") == "DolphinschedulerCluster"
        assert owners[0].get("name") == CLUSTER
        assert owners[0].get("controller") is True

    old_annotations = copy.deepcopy(object_annotations(old))
    new_annotations = copy.deepcopy(object_annotations(new))
    for annotations in (old_annotations, new_annotations):
        annotations.pop("banzaicloud.com/last-applied", None)
        annotations.pop("kubectl.kubernetes.io/last-applied-configuration", None)
    assert old_annotations == {}
    assert set(new_annotations) == {
        DB_INIT_CREDENTIALS_ANNOTATION,
        DB_INIT_IMAGE_ANNOTATION,
        DB_INIT_TARGET_ANNOTATION,
    }
    rollback_annotations = copy.deepcopy(object_annotations(rolled_back))
    rollback_annotations.pop("banzaicloud.com/last-applied", None)
    rollback_annotations.pop("kubectl.kubernetes.io/last-applied-configuration", None)
    assert rollback_annotations == {}

    old_meta = canonical_metadata(old)
    new_meta = canonical_metadata(new)
    old_meta.pop("annotations", None)
    new_meta.pop("annotations", None)
    assert new_meta == old_meta, "DB-init Job ownership/metadata drift"

    old_spec = copy.deepcopy(old["spec"])
    new_spec = copy.deepcopy(new["spec"])
    old_template = old_spec.pop("template")
    new_template = new_spec.pop("template")
    assert new_spec == old_spec, "DB-init Job retry/completion policy drift"
    expected_template_labels = legacy_cluster_labels() | {
        "batch.kubernetes.io/controller-uid": JOB_UID_TOKEN,
        "controller-uid": JOB_UID_TOKEN,
        "batch.kubernetes.io/job-name": CLUSTER,
        "job-name": CLUSTER,
    }
    assert old_template["metadata"].get("labels", {}) == expected_template_labels
    assert new_template["metadata"].get("labels", {}) == expected_template_labels
    old_template["metadata"].pop("labels", None)
    new_template["metadata"].pop("labels", None)
    assert new_template["metadata"] == old_template["metadata"]

    old_pod = old_template["spec"]
    new_pod = new_template["spec"]
    assert old_pod.get("securityContext") == new_pod.get("securityContext") == {
        "runAsGroup": 1001,
        "runAsNonRoot": False,
        "runAsUser": 1001,
    }
    old_pod.pop("securityContext")
    new_pod.pop("securityContext")
    assert old_pod.get("automountServiceAccountToken") in (None, True)
    assert new_pod.get("automountServiceAccountToken") is False
    old_pod.pop("automountServiceAccountToken", None)
    new_pod.pop("automountServiceAccountToken")

    old_init = old_pod.pop("initContainers")
    new_init = new_pod.pop("initContainers")
    assert len(old_init) == len(new_init) == 1
    assert old_init[0].get("command") == [
        "sh",
        "-xc",
        "for i in $(seq 1 180); do nc -z -w3 postgresql 5432 && exit 0 || sleep 5; done; exit 1",
    ]
    assert new_init[0].get("command") == [
        "sh",
        "-ec",
        'for i in $(seq 1 180); do nc -z -w3 "$1" "$2" && exit 0 || sleep 5; done; exit 1',
        "wait-for-database",
        "postgresql",
        "5432",
    ]
    for container in (old_init[0], new_init[0]):
        container.pop("command")
    assert new_init == old_init, "DB wait container drift beyond safe argv/xtrace migration"

    old_containers = old_pod.pop("containers")
    new_containers = new_pod.pop("containers")
    assert len(old_containers) == len(new_containers) == 1
    old_main = old_containers[0]
    new_main = new_containers[0]
    assert old_main.get("command") == ["/bin/bash", "-x", "-euo", "pipefail", "-c"]
    assert new_main.get("command") == ["/bin/bash", "-euo", "pipefail", "-c"]
    old_main.pop("command")
    new_main.pop("command")
    assert old_main.get("env", []) == []
    expected_credentials = [
        {
            "name": "SPRING_DATASOURCE_USERNAME",
            "valueFrom": {
                "secretKeyRef": {"key": "username", "name": "postgresql-credentials"}
            },
        },
        {
            "name": "SPRING_DATASOURCE_PASSWORD",
            "valueFrom": {
                "secretKeyRef": {"key": "password", "name": "postgresql-credentials"}
            },
        },
    ]
    assert encoded_counter(new_main.get("env", [])) == encoded_counter(expected_credentials)
    old_main.pop("env", None)
    new_main.pop("env", None)
    assert new_main == old_main, "DB schema container drift outside credential/xtrace migration"
    assert new_pod == old_pod, "DB-init pod drift outside token hardening/container migration"

    image = new_containers[0]["image"]
    assert new_annotations[DB_INIT_IMAGE_ANNOTATION] == image
    target = "\x00".join(("postgresql", "postgresql", "5432", "dolphinscheduler"))
    assert new_annotations[DB_INIT_TARGET_ANNOTATION] == hashlib.sha256(target.encode()).hexdigest()
    secret = json.loads(
        (root / "after-postgresql-credentials-identity.json").read_text()
    )
    credentials_version = (
        f"{NAMESPACE}/postgresql-credentials\x00{secret['resourceVersion']}"
    )
    assert new_annotations[DB_INIT_CREDENTIALS_ANNOTATION] == hashlib.sha256(
        credentials_version.encode()
    ).hexdigest()


def assert_after_semantic_whitelist(
    root: pathlib.Path,
    phases: dict[str, dict[tuple[str, str], dict]],
) -> None:
    """Require every Gen3 difference to match one documented migration rule."""

    before = phases["before"]
    after = phases["after"]
    dependency_fingerprint = expected_runtime_dependency_fingerprint(root, after)

    env_name = f"{CLUSTER}-envs"
    old_env = require(before, "ConfigMap", env_name)
    new_env = require(after, "ConfigMap", env_name)
    assert_labels(old_env, legacy_role_labels("master"), env_name)
    assert_labels(new_env, legacy_role_labels("master"), env_name)
    assert canonical_metadata(new_env) == canonical_metadata(old_env)
    expected_env_data = copy.deepcopy(old_env.get("data", {}))
    for key in ("SPRING_DATASOURCE_USERNAME", "SPRING_DATASOURCE_PASSWORD"):
        assert key in expected_env_data, (env_name, f"baseline missing {key}")
        expected_env_data.pop(key)
        assert key not in new_env.get("data", {}), (env_name, f"plaintext {key} survived")
    assert new_env.get("data", {}) == expected_env_data, (
        env_name,
        "environment ConfigMap changed beyond removing plaintext credentials",
    )
    for field in ("binaryData", "immutable"):
        assert new_env.get(field) == old_env.get(field), (env_name, f"ConfigMap {field} drift")

    for role in ROLES:
        name = f"{CLUSTER}-{role}-default"
        old_cm = require(before, "ConfigMap", name)
        new_cm = require(after, "ConfigMap", name)
        assert_labels(old_cm, legacy_role_labels(role), name)
        assert_labels(new_cm, framework_role_labels(role), name)
        assert canonical_metadata(new_cm) == canonical_metadata(old_cm)
        for field in ("binaryData", "data", "immutable"):
            assert new_cm.get(field) == old_cm.get(field), (name, f"ConfigMap {field} drift")

        old_kind = "StatefulSet" if role in {"master", "worker"} else "Deployment"
        old = require(before, old_kind, name)
        new = require(after, "StatefulSet", name)
        assert_labels(old, legacy_role_labels(role), name)
        assert_labels(new, framework_role_labels(role), name)
        assert canonical_metadata(new) == canonical_metadata(old), (name, "workload metadata drift")
        assert new["spec"].get("replicas") == old["spec"].get("replicas"), (name, "replica drift")
        assert_selector(old["spec"]["selector"], legacy_role_labels(role), name)
        assert_selector(new["spec"]["selector"], framework_selector(role), name)
        assert_pod_template_contract(old, new, role, dependency_fingerprint)
        assert new["spec"]["serviceName"] == name + "-headless", (
            name,
            "Gen3 StatefulSet does not use its dedicated headless Service",
        )
        assert_statefulset_policy(new, role)

        new_claims = new["spec"].get("volumeClaimTemplates", [])
        if role == "worker":
            assert new_claims == old["spec"].get("volumeClaimTemplates", []), (
                name,
                "worker claim-template drift",
            )
        else:
            assert not new_claims, (name, "unexpected Gen3 data claim")

        if old_kind == "StatefulSet":
            assert old["spec"]["serviceName"] == name, (
                name,
                "Gen2b StatefulSet no longer uses its legacy headless client Service",
            )
            assert old["spec"].get("podManagementPolicy") == "OrderedReady", (
                name,
                "Gen2b StatefulSet policy drift",
            )
            old_rest = copy.deepcopy(old["spec"])
            new_rest = copy.deepcopy(new["spec"])
            for spec in (old_rest, new_rest):
                for field in (
                    "podManagementPolicy",
                    "replicas",
                    "selector",
                    "serviceName",
                    "template",
                    "volumeClaimTemplates",
                ):
                    spec.pop(field, None)
            assert new_rest == old_rest, (name, "StatefulSet policy or storage drift")
        else:
            allowed_sts_keys = {
                "minReadySeconds",
                "ordinals",
                "persistentVolumeClaimRetentionPolicy",
                "podManagementPolicy",
                "replicas",
                "revisionHistoryLimit",
                "selector",
                "serviceName",
                "template",
                "updateStrategy",
            }
            assert set(new["spec"]) <= allowed_sts_keys, (
                name,
                "unclassified StatefulSet-only field",
                sorted(set(new["spec"]) - allowed_sts_keys),
            )

        assert_service_contract(before, after, role)

        old_pdb = require(before, "PodDisruptionBudget", f"{CLUSTER}-{role}")
        new_pdb = require(after, "PodDisruptionBudget", f"{CLUSTER}-{role}")
        assert_labels(old_pdb, legacy_role_labels(role, group=False), f"{CLUSTER}-{role}")
        assert_labels(new_pdb, framework_role_labels(role, group=False), f"{CLUSTER}-{role}")
        assert canonical_metadata(new_pdb) == canonical_metadata(old_pdb)
        assert_selector(old_pdb["spec"]["selector"], legacy_role_labels(role, group=False), role)
        assert_selector(new_pdb["spec"]["selector"], framework_selector(role, group=False), role)
        old_pdb_spec = copy.deepcopy(old_pdb["spec"])
        new_pdb_spec = copy.deepcopy(new_pdb["spec"])
        old_pdb_spec.pop("selector")
        new_pdb_spec.pop("selector")
        assert new_pdb_spec == old_pdb_spec, (role, "PDB budget/policy drift")

    assert_db_job_contract(root, before, after, phases["rollback"])
    assert_rbac_contract(before, after)


def normalize(item: dict) -> dict:
    value = (
        canonical_job_controller_identity(item)
        if item["kind"] == "Job"
        else copy.deepcopy(item)
    )
    value.pop("status", None)
    metadata = value["metadata"]
    for key in (
        "creationTimestamp",
        "generation",
        "managedFields",
        "resourceVersion",
        "uid",
    ):
        metadata.pop(key, None)
    annotations = metadata.get("annotations", {})
    annotations.pop("banzaicloud.com/last-applied", None)
    annotations.pop("deployment.kubernetes.io/revision", None)
    annotations.pop("kubectl.kubernetes.io/last-applied-configuration", None)
    if not annotations:
        metadata.pop("annotations", None)
    for owner in metadata.get("ownerReferences", []):
        owner.pop("uid", None)
    if value["kind"] == "Service":
        for key in ("clusterIP", "clusterIPs", "healthCheckNodePort"):
            value["spec"].pop(key, None)
        for port in value["spec"].get("ports", []):
            port.pop("nodePort", None)
    return value


def write_normalized(root: pathlib.Path, phase: str, items: dict) -> pathlib.Path:
    selected = [
        normalize(item)
        for _, item in sorted(items.items())
        if belongs_to_dolphinscheduler(item)
    ]
    path = root / f"{phase}-normalized.json"
    path.write_text(json.dumps(selected, indent=2, sort_keys=True) + "\n")
    return path


def write_diff(root: pathlib.Path, baseline: pathlib.Path, candidate: pathlib.Path, phase: str) -> str:
    diff = "".join(
        difflib.unified_diff(
            baseline.read_text().splitlines(True),
            candidate.read_text().splitlines(True),
            fromfile="before",
            tofile=phase,
        )
    )
    (root / f"{phase}.diff").write_text(diff)
    return diff


def main(root: pathlib.Path) -> None:
    phases = {
        phase: load_resources(root, phase)
        for phase in ("before", "after", "rollback")
    }
    assert_phase_contracts(phases)
    assert_exact_inventory(phases)
    assert_after_semantic_whitelist(root, phases)
    assert_pvc_identity(root)
    assert_db_init_job_identity(root)
    assert_dependency_identity(root)

    normalized = {
        phase: write_normalized(root, phase, items) for phase, items in phases.items()
    }
    after_diff = write_diff(root, normalized["before"], normalized["after"], "after")
    rollback_diff = write_diff(
        root, normalized["before"], normalized["rollback"], "rollback"
    )
    assert after_diff, "the allowlisted breaking Gen3 resource-shape change was not captured"
    assert not rollback_diff, (
        f"rollback did not restore the normalized baseline; inspect {root / 'rollback.diff'}"
    )
    print("Migration resource contracts passed; normalized diffs saved for review")


if __name__ == "__main__":
    main(pathlib.Path(sys.argv[1]))
