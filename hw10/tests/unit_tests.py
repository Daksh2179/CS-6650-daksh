"""
Unit Tests for Leader-Follower and Leaderless KV Store
CS 6650 - HW10
"""

import requests
import time
import random
import string
import threading
from datetime import datetime

# ─── Node Addresses ────────────────────────────────────────────────────────────

LEADER       = "http://localhost:8001"
FOLLOWERS    = [
    "http://localhost:8002",
    "http://localhost:8003",
    "http://localhost:8004",
    "http://localhost:8005",
]
LL_NODES     = [
    "http://localhost:9001",
    "http://localhost:9002",
    "http://localhost:9003",
    "http://localhost:9004",
    "http://localhost:9005",
]

# ─── Helpers ───────────────────────────────────────────────────────────────────

def random_key():
    return "key_" + "".join(random.choices(string.ascii_lowercase, k=6))

def kv_set(base_url, key, value):
    r = requests.post(f"{base_url}/set", json={"key": key, "value": value}, timeout=10)
    return r.status_code

def kv_get(base_url, key):
    r = requests.get(f"{base_url}/get", params={"key": key}, timeout=10)
    if r.status_code == 200:
        return r.json()
    return None

def kv_local_read(base_url, key):
    r = requests.get(f"{base_url}/local_read", params={"key": key}, timeout=10)
    if r.status_code == 200:
        return r.json()
    return None

def passed(name):
    print(f"  ✅ PASS: {name}")

def failed(name, reason):
    print(f"  ❌ FAIL: {name} -- {reason}")

# ─── Leader-Follower Tests ─────────────────────────────────────────────────────

def test_lf_write_read_leader():
    """After leader acknowledges write, read from leader must be consistent."""
    print("\n[LF Test 1] Write then read from leader")
    key = random_key()
    value = "lf_leader_value"

    status = kv_set(LEADER, key, value)
    assert status == 201, f"Expected 201, got {status}"

    result = kv_get(LEADER, key)
    if result and result["value"] == value:
        passed("Read from leader after write is consistent")
    else:
        failed("Read from leader after write", f"got {result}")


def test_lf_write_read_follower():
    """After leader acknowledges write, read from follower must be consistent."""
    print("\n[LF Test 2] Write then read from follower")
    key = random_key()
    value = "lf_follower_value"

    status = kv_set(LEADER, key, value)
    assert status == 201, f"Expected 201, got {status}"

    follower = random.choice(FOLLOWERS)
    result = kv_get(follower, key)
    if result and result["value"] == value:
        passed(f"Read from follower {follower} after write is consistent")
    else:
        failed(f"Read from follower {follower}", f"got {result}")


def test_lf_inconsistency_window():
    """
    Sneak a local_read to followers DURING replication.
    Since W=5 we won't catch inconsistency on the write ack,
    but we fire local_reads concurrently with the write to try.
    Run this many times at high load to catch inconsistency windows.
    """
    print("\n[LF Test 3] Inconsistency window via local_read during write")

    inconsistent_count = 0
    total_attempts = 20

    for i in range(total_attempts):
        key = random_key()
        value = f"val_{i}"
        stale_reads = []

        # Fire local_reads to all followers concurrently with the write
        def read_follower(follower_url):
            time.sleep(0.05)  # small delay to hit mid-replication
            result = kv_local_read(follower_url, key)
            stale_reads.append((follower_url, result))

        threads = [threading.Thread(target=read_follower, args=(f,)) for f in FOLLOWERS]
        for t in threads:
            t.start()

        # Fire the write
        kv_set(LEADER, key, value)

        for t in threads:
            t.join()

        # Check if any follower returned None (key not yet replicated = stale)
        for url, result in stale_reads:
            if result is None:
                inconsistent_count += 1
                print(f"    ⚠️  Stale read caught on {url} for key={key}")

    print(f"  Caught {inconsistent_count} stale local_reads out of {total_attempts * len(FOLLOWERS)} attempts")
    if inconsistent_count > 0:
        passed("Inconsistency window demonstrated via local_read")
    else:
        print("  ℹ️  No inconsistency caught (try increasing load or reducing W)")


def test_lf_version_ordering():
    """Write same key multiple times, final read should have highest version."""
    print("\n[LF Test 4] Version ordering -- last write wins")
    key = random_key()

    for i in range(5):
        kv_set(LEADER, key, f"value_{i}")

    result = kv_get(LEADER, key)
    if result and result["value"] == "value_4":
        passed(f"Last write wins, got value={result['value']} version={result['version']}")
    else:
        failed("Version ordering", f"got {result}")


# ─── Leaderless Tests ──────────────────────────────────────────────────────────

def test_ll_write_read_coordinator():
    """After coordinator acknowledges write, read from coordinator must be consistent."""
    print("\n[LL Test 1] Write then read from coordinator")
    key = random_key()
    value = "ll_coord_value"
    coordinator = random.choice(LL_NODES)

    status = kv_set(coordinator, key, value)
    assert status == 201, f"Expected 201, got {status}"

    result = kv_get(coordinator, key)
    if result and result["value"] == value:
        passed(f"Read from coordinator {coordinator} is consistent")
    else:
        failed(f"Read from coordinator {coordinator}", f"got {result}")


def test_ll_write_read_other_node():
    """After coordinator acknowledges write, read from another node must be consistent."""
    print("\n[LL Test 2] Write then read from different node")
    key = random_key()
    value = "ll_other_value"
    coordinator = LL_NODES[0]
    other_nodes = LL_NODES[1:]

    status = kv_set(coordinator, key, value)
    assert status == 201

    for node in other_nodes:
        result = kv_get(node, key)
        if result and result["value"] == value:
            passed(f"Read from {node} after write is consistent")
        else:
            failed(f"Read from {node}", f"got {result}")


def test_ll_inconsistency_window():
    """
    Fire reads to other nodes DURING replication window.
    Since internal_set sleeps 100ms + forwardSet sleeps 200ms,
    reads fired immediately after write starts should catch stale data.
    """
    print("\n[LL Test 3] Inconsistency window -- read during replication")

    inconsistent_count = 0
    total_attempts = 20

    for i in range(total_attempts):
        key = random_key()
        value = f"ll_val_{i}"
        coordinator = LL_NODES[0]
        stale_reads = []

        # Fire reads to other nodes almost immediately after sending write
        def read_node(node_url):
            time.sleep(0.05)  # hit during the 200ms replication delay
            result = kv_local_read(node_url, key)
            stale_reads.append((node_url, result))

        # Start reads concurrently
        threads = [threading.Thread(target=read_node, args=(n,)) for n in LL_NODES[1:]]
        for t in threads:
            t.start()

        # Fire write (this will take ~200ms+ to replicate)
        kv_set(coordinator, key, value)

        for t in threads:
            t.join()

        for url, result in stale_reads:
            if result is None:
                inconsistent_count += 1
                print(f"    ⚠️  Stale read on {url} for key={key}")

    print(f"  Caught {inconsistent_count} stale reads out of {total_attempts * len(LL_NODES[1:])} attempts")
    if inconsistent_count > 0:
        passed("Leaderless inconsistency window demonstrated")
    else:
        print("  ℹ️  No inconsistency caught -- try running under higher load")


def test_ll_all_nodes_consistent_after_write():
    """After write is fully acknowledged, ALL nodes must return consistent data."""
    print("\n[LL Test 4] All nodes consistent after write acknowledged")
    key = random_key()
    value = "ll_final_value"
    coordinator = random.choice(LL_NODES)

    status = kv_set(coordinator, key, value)
    assert status == 201

    all_consistent = True
    for node in LL_NODES:
        result = kv_get(node, key)
        if not result or result["value"] != value:
            failed(f"Node {node} inconsistent", f"got {result}")
            all_consistent = False

    if all_consistent:
        passed("All leaderless nodes consistent after write acknowledged")


# ─── Run All Tests ─────────────────────────────────────────────────────────────

if __name__ == "__main__":
    print("=" * 60)
    print("CS 6650 HW10 -- KV Store Unit Tests")
    print(f"Started at: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
    print("=" * 60)

    print("\n--- Leader-Follower Tests ---")
    test_lf_write_read_leader()
    test_lf_write_read_follower()
    test_lf_inconsistency_window()
    test_lf_version_ordering()

    print("\n--- Leaderless Tests ---")
    test_ll_write_read_coordinator()
    test_ll_write_read_other_node()
    test_ll_inconsistency_window()
    test_ll_all_nodes_consistent_after_write()

    print("\n" + "=" * 60)
    print("Tests complete.")
    print("=" * 60)