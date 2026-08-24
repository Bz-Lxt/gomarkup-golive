#!/usr/bin/env python3
"""Cost ¥0. Hits TCP REST only — never a metered API."""
from __future__ import annotations

import json
import sys
import urllib.error
import urllib.request

BASE = sys.argv[1] if len(sys.argv) > 1 else "http://127.0.0.1:19443"


def get(path: str) -> tuple[int, dict]:
    req = urllib.request.Request(BASE + path, headers={"Accept": "application/json"})
    try:
        with urllib.request.urlopen(req, timeout=5) as resp:
            body = json.loads(resp.read().decode())
            return resp.status, body
    except urllib.error.HTTPError as e:
        raw = e.read().decode()
        try:
            return e.code, json.loads(raw)
        except json.JSONDecodeError:
            return e.code, {"raw": raw}


def put(path: str, payload: dict) -> tuple[int, dict]:
    data = json.dumps(payload).encode()
    req = urllib.request.Request(
        BASE + path, data=data, method="PUT",
        headers={"Content-Type": "application/json"},
    )
    with urllib.request.urlopen(req, timeout=5) as resp:
        return resp.status, json.loads(resp.read().decode())


def main() -> int:
    failed = 0

    def check(name: str, cond: bool, detail: str = "") -> None:
        nonlocal failed
        mark = "PASS" if cond else "FAIL"
        if not cond:
            failed += 1
        print(f"[{mark}] {name} {detail}")

    code, health = get("/api/v1/health")
    check("health status field", "status" in health and "udp_listening" in health, str(health.get("status")))
    check("health not hardcoded-only", "cert_hours_remaining" in health)
    check("bbr layer disclosed", health.get("bbr_layer") == "application-scheduler")

    code, fp = get("/api/v1/cert-fingerprint")
    check("fingerprint http", code == 200)
    check("algorithm lowercase", fp.get("algorithm") == "sha-256")
    check("hex is leaf sha", isinstance(fp.get("hex"), str) and len(fp.get("hex", "")) == 64)
    check("wt url", str(fp.get("wt_url", "")).startswith("https://"))

    _, cfg = get("/api/v1/config")
    check("channel matrix", isinstance(cfg.get("channels"), list) and len(cfg["channels"]) == 5)

    code, snap = put("/api/v1/netem", {"preset": "30"})
    check("netem hot switch", code == 200 and snap.get("uplink", {}).get("loss_pct") == 30)
    put("/api/v1/netem", {"preset": "0"})

    try:
        put("/api/v1/netem", {"preset": "99"})
        check("bad preset rejected", False)
    except urllib.error.HTTPError as e:
        check("bad preset rejected", e.code == 422)

    print(f"cost=¥0 failed={failed}")
    return 1 if failed else 0


if __name__ == "__main__":
    raise SystemExit(main())
