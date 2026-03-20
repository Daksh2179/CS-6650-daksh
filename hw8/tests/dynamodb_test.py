import requests
import json
import time
from datetime import datetime, timezone

BASE_URL = "http://hw8-alb-530893878.us-west-2.elb.amazonaws.com"
RESULTS_FILE = "../results/dynamodb_test_results.json"

results = []
cart_ids = []

def record(operation, response, start):
    elapsed = (time.time() - start) * 1000  # ms
    results.append({
        "operation": operation,
        "response_time": round(elapsed, 2),
        "success": response.status_code in [200, 201],
        "status_code": response.status_code,
        "timestamp": datetime.now(timezone.utc).isoformat()
    })

print("Starting DynamoDB test - 150 operations (50 create, 50 add, 50 get)")

# 50 create cart
print("Running 50 POST /dynamo/shopping-carts...")
for i in range(50):
    start = time.time()
    r = requests.post(f"{BASE_URL}/dynamo/shopping-carts",
                      json={"customer_id": f"customer-{i}"},
                      timeout=30)
    record("create_cart", r, start)
    if r.status_code == 201:
        cart_ids.append(r.json()["id"])

# 50 add items
print("Running 50 POST /dynamo/shopping-carts/:id/items...")
for i in range(50):
    cart_id = cart_ids[i % len(cart_ids)]
    start = time.time()
    r = requests.post(f"{BASE_URL}/dynamo/shopping-carts/{cart_id}/items",
                      json={"product_id": f"prod-{i}", "quantity": i + 1, "price": round((i + 1) * 4.99, 2)},
                      timeout=30)
    record("add_items", r, start)

# 50 get cart
print("Running 50 GET /dynamo/shopping-carts/:id...")
for i in range(50):
    cart_id = cart_ids[i % len(cart_ids)]
    start = time.time()
    r = requests.get(f"{BASE_URL}/dynamo/shopping-carts/{cart_id}", timeout=30)
    record("get_cart", r, start)

# Save results
with open(RESULTS_FILE, "w") as f:
    json.dump(results, f, indent=2)

# Summary
total = len(results)
success = sum(1 for r in results if r["success"])
avg_rt = sum(r["response_time"] for r in results) / total

print(f"\nDone! {total} operations, {success} successful ({round(success/total*100, 1)}%)")
print(f"Average response time: {round(avg_rt, 2)}ms")
print(f"Results saved to {RESULTS_FILE}")