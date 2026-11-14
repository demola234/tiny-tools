#!/usr/bin/env python3
"""Prints Markdown tables from a results.tsv written by tool/run_all.sh.

Usage: tool/tables.py [build/bench/results.tsv]
"""
import csv
import sys

OPS = ["write 16 B", "read 16 B", "write 1 KiB", "read 1 KiB", "delete", "read all"]
COLD = ["open (ms)", "ready (ms)", "first read (µs)", "read every key (ms)", "RSS growth (MiB)"]
PLAIN = ["rodinia", "rodinia (no cache)", "hive_ce", "shared_preferences", "sqflite", "get_storage"]
ENCRYPTED = ["rodinia (AES-GCM)", "hive_ce (AES-CBC)", "flutter_secure_storage"]
COLD_STORES = ["rodinia", "hive_ce", "shared_preferences", "sqflite", "get_storage", "flutter_secure_storage"]


def load(path):
    results = {}
    with open(path, newline="") as f:
        for row in csv.DictReader(f, delimiter="\t"):
            results.setdefault(row["store"], {})[row["metric"]] = float(row["value"])
    return results


def fmt(value):
    if value >= 1000:
        return f"{value:,.0f}"
    return f"{value:.1f}" if value >= 10 else f"{value:.2f}"


def table(title, note, results, stores, metrics):
    stores = [s for s in stores if all(m in results.get(s, {}) for m in metrics)]
    lines = [f"### {title}", "", note, ""]
    lines.append("| | " + " | ".join(f"`{s}`" for s in stores) + " |")
    lines.append("|---|" + "---:|" * len(stores))
    for metric in metrics:
        values = [results[s][metric] for s in stores]
        best = min(values)
        cells = [f"**{fmt(v)}**" if v == best else fmt(v) for v in values]
        lines.append(f"| {metric} | " + " | ".join(cells) + " |")
    return "\n".join(lines)


def main():
    results = load(sys.argv[1] if len(sys.argv) > 1 else "build/bench/results.tsv")
    per_op = "Median µs per operation, lower is better. Fastest in bold."
    print(table("Plain storage, 500 keys", per_op, results, PLAIN, OPS))
    print()
    print(table("Encrypted storage, 500 keys (flutter_secure_storage: 100)", per_op, results, ENCRYPTED, OPS))
    print()
    print(table(
        "Cold start, 1,000 × 1 KiB, fresh process per store",
        "One sample per store, lower is better. Lowest in bold. *ready* is when "
        "background loading after open has finished; only rodinia loads in the "
        "background (its preload), so for the others it equals *open*. Reads "
        "start after *ready*.",
        results, COLD_STORES, COLD,
    ))


if __name__ == "__main__":
    main()
