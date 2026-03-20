import json
import statistics

MYSQL_FILE    = "../results/mysql_test_results.json"
DYNAMO_FILE   = "../results/dynamodb_test_results.json"
COMBINED_FILE = "../results/combined_results.json"

with open(MYSQL_FILE)  as f: mysql_results  = json.load(f)
with open(DYNAMO_FILE) as f: dynamo_results = json.load(f)

# Tag each record with its database
for r in mysql_results:  r["database"] = "mysql"
for r in dynamo_results: r["database"] = "dynamodb"

combined = mysql_results + dynamo_results

with open(COMBINED_FILE, "w") as f:
    json.dump(combined, f, indent=2)

print(f"combined_results.json written ({len(combined)} total records)\n")

def analyze(results, label):
    times = [r["response_time"] for r in results]
    success = sum(1 for r in results if r["success"])
    times_sorted = sorted(times)
    n = len(times_sorted)

    def percentile(data, p):
        idx = int(len(data) * p / 100)
        return data[min(idx, len(data)-1)]

    print(f"=== {label} ({len(results)} ops) ===")
    print(f"  Success rate : {round(success/len(results)*100, 1)}%")
    print(f"  Avg          : {round(statistics.mean(times), 2)}ms")
    print(f"  P50          : {round(percentile(times_sorted, 50), 2)}ms")
    print(f"  P95          : {round(percentile(times_sorted, 95), 2)}ms")
    print(f"  P99          : {round(percentile(times_sorted, 99), 2)}ms")
    print(f"  Min          : {round(min(times), 2)}ms")
    print(f"  Max          : {round(max(times), 2)}ms")

    for op in ["create_cart", "add_items", "get_cart"]:
        op_times = [r["response_time"] for r in results if r["operation"] == op]
        if op_times:
            print(f"  {op:12} avg: {round(statistics.mean(op_times), 2)}ms")
    print()

analyze(mysql_results,  "MySQL")
analyze(dynamo_results, "DynamoDB")

# Comparison table
def avg(results): return round(statistics.mean([r["response_time"] for r in results]), 2)
def pct(results, p):
    t = sorted([r["response_time"] for r in results])
    return round(t[int(len(t)*p/100)], 2)

print("=== COMPARISON TABLE ===")
print(f"{'Metric':<25} {'MySQL':>10} {'DynamoDB':>10} {'Winner':>10}")
print("-" * 60)

metrics = [
    ("Avg (ms)",      avg(mysql_results),         avg(dynamo_results)),
    ("P50 (ms)",      pct(mysql_results, 50),      pct(dynamo_results, 50)),
    ("P95 (ms)",      pct(mysql_results, 95),      pct(dynamo_results, 95)),
    ("P99 (ms)",      pct(mysql_results, 99),      pct(dynamo_results, 99)),
    ("Success Rate",  f"{round(sum(1 for r in mysql_results if r['success'])/150*100,1)}%",
                      f"{round(sum(1 for r in dynamo_results if r['success'])/150*100,1)}%"),
]

for name, m, d in metrics:
    if isinstance(m, float) and isinstance(d, float):
        winner = "DynamoDB" if d < m else "MySQL"
        margin = abs(round(m - d, 2))
        print(f"{name:<25} {m:>10} {d:>10} {winner:>10}  (margin: {margin}ms)")
    else:
        print(f"{name:<25} {str(m):>10} {str(d):>10} {'Tie':>10}")

print(f"\nOperation breakdown:")
print(f"{'Operation':<15} {'MySQL avg':>12} {'DynamoDB avg':>12} {'Faster by':>12}")
print("-" * 55)
for op in ["create_cart", "add_items", "get_cart"]:
    m_times = [r["response_time"] for r in mysql_results  if r["operation"] == op]
    d_times = [r["response_time"] for r in dynamo_results if r["operation"] == op]
    m_avg = round(statistics.mean(m_times), 2)
    d_avg = round(statistics.mean(d_times), 2)
    faster = "DynamoDB" if d_avg < m_avg else "MySQL"
    margin = abs(round(m_avg - d_avg, 2))
    print(f"{op:<15} {m_avg:>12} {d_avg:>12} {faster+' +'+str(margin)+'ms':>12}")