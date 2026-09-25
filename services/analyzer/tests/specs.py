"""Fixture builders for the recommendation rules (not collected: no test_ prefix).

`hardened()` is the V3 hardened base: a Deployment every v1 rule accepts, with its
Service, PodDisruptionBudget and NetworkPolicy, two Running pods on two nodes and every
input family complete. Tests change one field of it and rebuild: pods always mirror the
template (effective resources), unless the test edits the pods themselves.
"""

import copy
from datetime import UTC, datetime

from models import AppInsights
from review import ReviewInputs, WorkloadInput

NOW = datetime(2026, 9, 24, 12, 0, tzinfo=UTC)
NS = "shop"
DIGEST = "sha256:" + "a" * 64
IMAGE = f"nginxinc/nginx-unprivileged:1.27-alpine@{DIGEST}"
ALL = {"W", "S", "P", "H", "N", "U", "A", "D", "X"}


def container(name="app", **over):
    c = {
        "name": name, "image": IMAGE, "imagePullPolicy": "IfNotPresent",
        "ports": [{"name": "http", "containerPort": 8080}],
        "readinessProbe": {"httpGet": {"path": "/", "port": "http"}, "periodSeconds": 5},
        "livenessProbe": {"tcpSocket": {"port": "http"}, "periodSeconds": 10, "failureThreshold": 3},
        "resources": {"requests": {"cpu": "20m", "memory": "32Mi"}, "limits": {"cpu": "100m", "memory": "64Mi"}},
        "securityContext": {"runAsNonRoot": True, "runAsUser": 101, "allowPrivilegeEscalation": False,
                            "readOnlyRootFilesystem": True, "capabilities": {"drop": ["ALL"]},
                            "seccompProfile": {"type": "RuntimeDefault"}},
        "volumeMounts": [{"name": "tmp", "mountPath": "/tmp"}],
    }
    c.update(over)
    return c


def workload(name="web", kind="Deployment", replicas=2, containers=None, **pod_over):
    pod_spec = {
        "containers": containers or [container()],
        "volumes": [{"name": "tmp", "emptyDir": {}}],
        "serviceAccountName": name, "automountServiceAccountToken": False,
        "topologySpreadConstraints": [{"maxSkew": 1, "topologyKey": "kubernetes.io/hostname",
                                       "whenUnsatisfiable": "ScheduleAnyway"}],
        "terminationGracePeriodSeconds": 30,
    }
    pod_spec.update(pod_over)
    spec = {"selector": {"matchLabels": {"app.kubernetes.io/name": name}},
            "template": {"metadata": {"labels": {"app.kubernetes.io/name": name}}, "spec": pod_spec}}
    if kind != "DaemonSet":
        spec["replicas"] = replicas
    if kind == "Deployment":
        spec["strategy"] = {"type": "RollingUpdate", "rollingUpdate": {"maxUnavailable": 0, "maxSurge": 1}}
    return {"kind": kind, "metadata": {"name": name, "namespace": NS}, "spec": spec}


def pods(obj, count=None, nodes=("n1", "n2"), phase="Running", restarts=0, token=False):
    """Pods mirroring the template (effective resources), round-robin over `nodes`."""
    spec = obj["spec"]
    count = spec.get("replicas", 2) if count is None else count
    pod_spec = copy.deepcopy(spec["template"]["spec"])
    if token:
        pod_spec.setdefault("volumes", []).append({"name": "kube-api-access-x", "projected": {"sources": [
            {"serviceAccountToken": {"expirationSeconds": 3607, "path": "token"}}]}})
    out = []
    for i in range(count):
        s = copy.deepcopy(pod_spec)
        s["nodeName"] = nodes[i % len(nodes)]
        out.append({"metadata": {"name": f"{obj['metadata']['name']}-7d9f8b6c5-x2k4{'bcdfghjklm'[i]}"}, "spec": s,
                    "status": {"phase": phase, "qosClass": "Burstable", "containerStatuses": [
                        {"name": c["name"], "restartCount": restarts} for c in s["containers"]]}})
    return out


def service(name="web", selector=None, target="http", ns=NS, **spec):
    return {"metadata": {"name": name, "namespace": ns},
            "spec": {"selector": {"app.kubernetes.io/name": name} if selector is None else selector,
                     "ports": [{"port": 80, "targetPort": target}], **spec}}


def pdb(name="web", status=None, **spec):
    body = {"selector": {"matchLabels": {"app.kubernetes.io/name": name}}}
    body.update(spec or {"maxUnavailable": 1})
    return {"metadata": {"name": name}, "spec": body,
            "status": status or {"disruptionsAllowed": 1, "currentHealthy": 2, "expectedPods": 2}}


def hpa(target="web", min_replicas=1, max_replicas=2, metrics=None, status=None, kind="Deployment", behavior=None):
    spec = {"scaleTargetRef": {"kind": kind, "name": target}, "minReplicas": min_replicas, "maxReplicas": max_replicas,
            "metrics": metrics if metrics is not None else [
                {"type": "Resource", "resource": {"name": "cpu", "target": {"type": "Utilization",
                                                                            "averageUtilization": 50}}}]}
    if behavior:
        spec["behavior"] = behavior
    return {"metadata": {"name": target}, "spec": spec,
            "status": status or {"currentReplicas": min_replicas, "conditions": []}}


def netpol(name="web", selector=None, ingress=None):
    return {"metadata": {"name": name}, "spec": {
        "podSelector": {"matchLabels": {"app.kubernetes.io/name": name}} if selector is None else selector,
        "policyTypes": ["Ingress"],
        "ingress": ingress if ingress is not None else [{"from": [{"podSelector": {}}]}]}}


def usage(samples):
    """samples: [{container: (cpu_m, mem_b)}], one hour apart."""
    return [{"t": 1790000000 + 3600 * i, "containers": {c: {"cpu_m": v[0], "mem_b": v[1]} for c, v in s.items()}}
            for i, s in enumerate(samples)]


def build(objs=None, ns=NS, services=None, pdbs=None, hpas=None, netpols=None, usage_by=None, complete=ALL,
          doc=None, events=None, plans=None, envs=None, selector_pods=None, app=None, pod_lists=None,
          extra_resources=(), namespaces=None, capped=False):
    objs = objs if objs is not None else [workload()]
    names = [o["metadata"]["name"] for o in objs]
    namespaces = namespaces or [ns]
    app = app or {
        "name": names[0] if names else "app",
        "namespaces": {"items": [{"name": n} for n in namespaces]},
        "resources": [{"namespace": o["metadata"].get("namespace", ns), "kind": o["kind"], "name": o["metadata"]["name"]}
                      for o in objs] + [{"namespace": ns, "kind": "Service", "name": n} for n in names]
        + list(extra_resources),
        "metrics": {}, "history": {"changeLog": []},
    }
    inputs = ReviewInputs(app=app, doc=doc or AppInsights(), namespaces=namespaces)
    for i, obj in enumerate(objs):
        kind = obj["kind"].lower()
        w_ns = obj["metadata"].get("namespace", ns)
        wl_pods = pod_lists[i] if pod_lists is not None else pods(obj, count=None if kind != "daemonset" else 2)
        subject = f"{kind}/{obj['metadata']['name']}"
        inputs.workloads[(w_ns, subject)] = WorkloadInput(kind, obj["metadata"]["name"], w_ns, subject, obj, wl_pods)
    default = {
        "S": services if services is not None else [service(n) for n in names],
        "P": pdbs if pdbs is not None else [pdb(n) for n in names],
        "H": hpas if hpas is not None else [],
        "N": netpols if netpols is not None else [netpol(n) for n in names],
    }
    inputs.lists = {family: {n: list(items) for n in namespaces} for family, items in default.items()}
    inputs.selector_pods = selector_pods if selector_pods is not None else {(ns, n): 1 for n in names}
    inputs.usage = usage_by or {}
    inputs.plans = plans if plans is not None else []
    inputs.env_names = envs or {}
    inputs.events = events or []
    inputs.complete = set(complete) | ({"E"} if events is not None else set())
    inputs.capped = capped
    return inputs


def hardened(name="web"):
    return build([workload(name)])
