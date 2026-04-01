"""
Load Tester for Leader-Follower and Leaderless KV Store
CS 6650 - HW10

Configurations tested:
  1. LF W=5 R=1
  2. LF W=1 R=5
  3. LF W=3 R=3
  4. Leaderless W=N R=1

Read/write ratios tested:
  1%/99%, 10%/90%, 50%/50%, 90%/10%
"""

import requests
import time
import random
import string
import threading
import os
import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
from datetime import datetime
from collections import defaultdict

# --- Node Addresses ---

LEADER = "http://localhost:8001"
FOLLOWERS = [
    "http://localhost:8002",
    "http://localhost:8003",
    "http://localhost:8004",
    "http://localhost:8005",
]
ALL_LF_NODES = [LEADER] + FOLLOWERS

LL_NODES = [
    "http://localhost:9001",
    "http://localhost:9002",
    "http://localhost:9003",
    "http://localhost:9004",
    "http://localhost:9005",
]

# --- Config ---

TOTAL_REQUESTS = 500       # per ratio per config
NUM_KEYS = 20              # small key space forces reads/writes to same keys
NUM_THREADS = 10

# Set this via command line arg: python load_tester.py W5R1 or W1R5 or W3R3 or LL
# When running all at once (local test), set to "ALL"
import sys
RUN_CONFIG = sys.argv[1] if len(sys.argv) > 1 else "ALL"

RATIOS = [
    (0.01, 0.99),
    (0.10, 0.90),
    (0.50, 0.50),
    (0.90, 0.10),
]

CONFIGS = [
    {"name": "LF_W5_R1",    "write_url": LEADER,                "read_urls": ALL_LF_NODES, "type": "lf"},
    {"name": "LF_W1_R5",    "write_url": LEADER,                "read_urls": ALL_LF_NODES, "type": "lf"},
    {"name": "LF_W3_R3",    "write_url": LEADER,                "read_urls": ALL_LF_NODES, "type": "lf"},
    {"name": "Leaderless",  "write_url": None,                  "read_urls": LL_NODES,     "type": "ll"},
]

# --- Helpers ---

def random_key():
    return f"k{random.randint(0, NUM_KEYS - 1)}"

def random_value():
    return "".join(random.choices(string.ascii_lowercase, k=6))

def do_write(write_url, key, value):
    if write_url is None:
        write_url = random.choice(LL_NODES)
    start = time.time()
    try:
        r = requests.post(f"{write_url}/set", json={"key": key, "value": value}, timeout=10)
        latency = (time.time() - start) * 1000
        return latency, r.status_code == 201, value
    except Exception:
        return (time.time() - start) * 1000, False, value

def do_read(read_urls, key, expected_value, expected_version):
    url = random.choice(read_urls)
    start = time.time()
    try:
        r = requests.get(f"{url}/get", params={"key": key}, timeout=10)
        latency = (time.time() - start) * 1000
        if r.status_code == 200:
            data = r.json()
            is_stale = (
                expected_version is not None and
                data.get("version", -1) < expected_version
            )
            return latency, True, is_stale
        return latency, False, False
    except Exception:
        return (time.time() - start) * 1000, False, False

# --- Load Test Runner ---

def run_load_test(config, write_ratio, read_ratio):
    write_latencies = []
    read_latencies = []
    stale_reads = 0
    total_reads = 0

    # Track latest written version per key so we can detect stale reads
    key_state = {}  # key -> {"value": ..., "version": ..., "write_time": ...}
    key_state_lock = threading.Lock()

    # Track time intervals between write and subsequent read of same key
    rw_intervals = []

    n_writes = int(TOTAL_REQUESTS * write_ratio)
    n_reads = int(TOTAL_REQUESTS * read_ratio)

    # Build operation list: cluster reads near writes for same keys
    ops = []
    keys_pool = [random_key() for _ in range(max(1, n_writes))]

    for key in keys_pool:
        ops.append(("write", key))
        # Add nearby reads for the same key right after the write
        nearby = max(1, int(n_reads / max(1, n_writes)))
        for _ in range(min(nearby, 5)):
            ops.append(("read", key))

    # Fill remaining with random key reads
    while sum(1 for o in ops if o[0] == "read") < n_reads:
        ops.append(("read", random_key()))

    while sum(1 for o in ops if o[0] == "write") < n_writes:
        ops.append(("write", random_key()))

    random.shuffle(ops)

    results_lock = threading.Lock()

    def execute_op(op):
        nonlocal stale_reads, total_reads
        op_type, key = op

        if op_type == "write":
            value = random_value()
            latency, ok, val = do_write(config["write_url"], key, value)
            if ok:
                with key_state_lock:
                    prev = key_state.get(key, {})
                    new_version = prev.get("version", 0) + 1
                    key_state[key] = {
                        "value": val,
                        "version": new_version,
                        "write_time": time.time()
                    }
            with results_lock:
                write_latencies.append(latency)

        else:
            with key_state_lock:
                state = key_state.get(key)
            if state is None:
                return
            expected_version = state["version"]
            write_time = state["write_time"]

            latency, ok, is_stale = do_read(config["read_urls"], key, state["value"], expected_version)

            read_time = time.time()
            interval_ms = (read_time - write_time) * 1000

            with results_lock:
                total_reads += 1
                read_latencies.append(latency)
                rw_intervals.append(interval_ms)
                if is_stale:
                    stale_reads += 1

    threads = []
    for op in ops:
        t = threading.Thread(target=execute_op, args=(op,))
        threads.append(t)
        if len(threads) >= NUM_THREADS:
            for th in threads:
                th.start()
            for th in threads:
                th.join()
            threads = []

    for t in threads:
        t.start()
    for t in threads:
        t.join()

    return {
        "write_latencies": write_latencies,
        "read_latencies": read_latencies,
        "stale_reads": stale_reads,
        "total_reads": total_reads,
        "rw_intervals": rw_intervals,
    }

# --- Graphing ---

def save_graphs(all_results, output_dir):
    os.makedirs(output_dir, exist_ok=True)

    ratio_labels = ["1pct_write", "10pct_write", "50pct_write", "90pct_write"]
    ratio_display = ["1%/99%", "10%/90%", "50%/50%", "90%/10%"]

    for ri, (wr, rr) in enumerate(RATIOS):
        ratio_key = ratio_labels[ri]
        fig, axes = plt.subplots(2, len(CONFIGS), figsize=(5 * len(CONFIGS), 8))
        fig.suptitle(f"Latency Distribution - Write/Read Ratio {ratio_display[ri]}", fontsize=13)

        for ci, config in enumerate(CONFIGS):
            res = all_results[config["name"]][ratio_key]
            rl = res["read_latencies"]
            wl = res["write_latencies"]

            ax_r = axes[0][ci]
            ax_w = axes[1][ci]

            if rl:
                ax_r.hist(rl, bins=30, color="steelblue", edgecolor="white")
                ax_r.set_title(f"{config['name']} - Reads")
                ax_r.set_xlabel("Latency (ms)")
                ax_r.set_ylabel("Count")

            if wl:
                ax_w.hist(wl, bins=30, color="coral", edgecolor="white")
                ax_w.set_title(f"{config['name']} - Writes")
                ax_w.set_xlabel("Latency (ms)")
                ax_w.set_ylabel("Count")

        plt.tight_layout()
        config_tag = RUN_CONFIG if RUN_CONFIG != "ALL" else "ALL"
        path = os.path.join(output_dir, f"{config_tag}_latency_{ratio_key}.png")
        plt.savefig(path)
        plt.close()
        print(f"  Saved: {path}")

    # RW interval distribution per config
    configs_to_run = [c for c in CONFIGS if RUN_CONFIG == "ALL" or c["name"] == RUN_CONFIG]

    for config in configs_to_run:
        fig, axes = plt.subplots(1, len(RATIOS), figsize=(5 * len(RATIOS), 4))
        fig.suptitle(f"Read-Write Interval Distribution - {config['name']}", fontsize=13)

        for ri, (wr, rr) in enumerate(RATIOS):
            ratio_key = ratio_labels[ri]
            intervals = all_results[config["name"]][ratio_key]["rw_intervals"]
            ax = axes[ri]
            if intervals:
                ax.hist(intervals, bins=30, color="mediumpurple", edgecolor="white")
                ax.set_title(f"Ratio {ratio_display[ri]}")
                ax.set_xlabel("Interval (ms)")
                ax.set_ylabel("Count")

        plt.tight_layout()
        config_tag = RUN_CONFIG if RUN_CONFIG != "ALL" else "ALL"
        path = os.path.join(output_dir, f"{config_tag}_rw_interval_{config['name']}.png")
        plt.savefig(path)
        plt.close()
        print(f"  Saved: {path}")

# --- Summary Printer ---

def print_summary(all_results):
    ratio_labels = ["1pct_write", "10pct_write", "50pct_write", "90pct_write"]
    ratio_display = ["1%W/99%R", "10%W/90%R", "50%W/50%R", "90%W/10%R"]

    print("\n" + "=" * 70)
    print("LOAD TEST SUMMARY")
    print("=" * 70)

    for config in CONFIGS:
        print(f"\nConfig: {config['name']}")
        for ri, rk in enumerate(ratio_labels):
            res = all_results[config["name"]][rk]
            rl = res["read_latencies"]
            wl = res["write_latencies"]
            stale = res["stale_reads"]
            total_r = res["total_reads"]

            avg_r = sum(rl) / len(rl) if rl else 0
            avg_w = sum(wl) / len(wl) if wl else 0
            max_r = max(rl) if rl else 0
            max_w = max(wl) if wl else 0
            stale_pct = (stale / total_r * 100) if total_r > 0 else 0

            print(f"  Ratio {ratio_display[ri]}:")
            print(f"    Reads  - avg: {avg_r:.1f}ms  max: {max_r:.1f}ms  count: {len(rl)}")
            print(f"    Writes - avg: {avg_w:.1f}ms  max: {max_w:.1f}ms  count: {len(wl)}")
            print(f"    Stale reads: {stale}/{total_r} ({stale_pct:.1f}%)")

# --- Main ---

if __name__ == "__main__":
    print("=" * 70)
    print("CS 6650 HW10 - Load Tester")
    print(f"Started: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
    print(f"Requests per run: {TOTAL_REQUESTS}, Keys: {NUM_KEYS}, Threads: {NUM_THREADS}")
    print("=" * 70)

    all_results = defaultdict(dict)
    ratio_labels = ["1pct_write", "10pct_write", "50pct_write", "90pct_write"]

    for config in CONFIGS:
        print(f"\nRunning config: {config['name']}")
        for ri, (wr, rr) in enumerate(RATIOS):
            label = ratio_labels[ri]
            print(f"  Ratio {int(wr*100)}%W / {int(rr*100)}%R ...", end=" ", flush=True)
            result = run_load_test(config, wr, rr)
            all_results[config["name"]][label] = result
            print(f"done. Writes: {len(result['write_latencies'])}, Reads: {len(result['read_latencies'])}, Stale: {result['stale_reads']}")

    print_summary(all_results)

    print("\nSaving graphs...")
    output_dir = os.path.join(os.path.dirname(__file__), "..", "report")
    save_graphs(all_results, output_dir)

    print("\nDone. Graphs saved to hw10/report/")