#!/usr/bin/env python3
"""Telemetry Context Injector — calls /api/brain-scan and produces a ~150-word
natural-language paragraph describing the current graph state.

Output is meant to be injected into the EVA System Prompt under [ESTADO DO GRAFO].

Usage:
    python telemetry_injector.py                          # default VM
    python telemetry_injector.py --host http://localhost:8080
    python telemetry_injector.py --json                   # raw JSON instead of text
"""

from __future__ import annotations

import argparse
import json
import sys
import urllib.request
import urllib.error

DEFAULT_HOST = "http://136.111.0.47:8080"


def fetch_brain_scan(host: str, timeout: float = 5.0) -> dict:
    url = f"{host}/api/brain-scan"
    req = urllib.request.Request(url, headers={"Accept": "application/json"})
    with urllib.request.urlopen(req, timeout=timeout) as resp:
        return json.loads(resp.read().decode())


def format_paragraph(scan: dict) -> str:
    """Convert brain-scan JSON into a concise natural-language paragraph (~150 words)."""
    total_n = scan.get("total_nodes", 0)
    total_e = scan.get("total_edges", 0)
    n_cols = scan.get("total_collections", 0)
    uptime_s = scan.get("uptime_secs", 0)
    ram_mb = scan.get("ram_usage_mb", 0)

    # Top collections by node count
    cols = sorted(scan.get("collections", []), key=lambda c: c.get("node_count", 0), reverse=True)
    top3 = cols[:3]
    top_desc = ", ".join(
        f"{c['name']} ({c['node_count']} nós/{c.get('edge_count', 0)} edges)"
        for c in top3
    )

    # PageRank highlights
    pr = scan.get("pagerank_top5", [])
    if pr:
        pr_desc = ", ".join(f'"{e["label"]}"' for e in pr[:5])
        pr_line = f"Os conceitos mais influentes são: {pr_desc}."
    else:
        pr_line = "Sem dados de PageRank disponíveis."

    # Uptime formatting
    if uptime_s < 3600:
        uptime_str = f"{uptime_s // 60} minutos"
    else:
        h = uptime_s // 3600
        m = (uptime_s % 3600) // 60
        uptime_str = f"{h}h{m:02d}m"

    # Empty collections
    empty = [c["name"] for c in cols if c.get("node_count", 0) == 0]
    empty_line = ""
    if empty:
        empty_line = f" Collections vazias: {', '.join(empty[:5])}."

    paragraph = (
        f"O meu grafo de conhecimento contém {total_n:,} nós e {total_e:,} edges "
        f"distribuídos por {n_cols} collections. "
        f"As maiores são: {top_desc}. "
        f"{pr_line} "
        f"O servidor está activo há {uptime_str}, a usar {ram_mb} MB de RAM."
        f"{empty_line}"
    )
    return paragraph.strip()


def main():
    parser = argparse.ArgumentParser(description="EVA Telemetry Context Injector")
    parser.add_argument("--host", default=DEFAULT_HOST, help="NietzscheDB HTTP base URL")
    parser.add_argument("--json", action="store_true", help="Output raw JSON")
    parser.add_argument("--timeout", type=float, default=5.0, help="HTTP timeout in seconds")
    args = parser.parse_args()

    try:
        scan = fetch_brain_scan(args.host, timeout=args.timeout)
    except urllib.error.URLError as e:
        print(f"[ERRO] Não consegui contactar {args.host}/api/brain-scan: {e}", file=sys.stderr)
        sys.exit(1)
    except json.JSONDecodeError as e:
        print(f"[ERRO] Resposta inválida do brain-scan: {e}", file=sys.stderr)
        sys.exit(1)

    if args.json:
        print(json.dumps(scan, indent=2))
    else:
        print(format_paragraph(scan))


if __name__ == "__main__":
    main()
