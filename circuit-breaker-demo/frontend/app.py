from flask import Flask, jsonify
import requests
import time

app = Flask(__name__)

BACKEND_URL = "http://backend:5001"

# Circuit Breaker State
circuit = {
    "state": "CLOSED",       # CLOSED = normal, OPEN = blocking requests, HALF_OPEN = testing
    "failures": 0,
    "threshold": 3,           # Open circuit after 3 failures
    "last_failure_time": 0,
    "recovery_timeout": 10    # Try recovery after 10 seconds
}

def call_backend():
    now = time.time()

    # OPEN state - circuit is tripped
    if circuit["state"] == "OPEN":
        if now - circuit["last_failure_time"] > circuit["recovery_timeout"]:
            circuit["state"] = "HALF_OPEN"
            print("Circuit HALF_OPEN - testing backend...")
        else:
            raise Exception("Circuit is OPEN - request blocked")

    try:
        response = requests.get(f"{BACKEND_URL}/data", timeout=5)
        if response.status_code != 200:
            raise Exception("Backend returned error")

        # Success - reset circuit
        circuit["failures"] = 0
        circuit["state"] = "CLOSED"
        return response.json()

    except Exception as e:
        circuit["failures"] += 1
        circuit["last_failure_time"] = time.time()
        if circuit["failures"] >= circuit["threshold"]:
            circuit["state"] = "OPEN"
            print(f"Circuit OPEN after {circuit['failures']} failures")
        raise e

@app.route('/product')
def get_product():
    try:
        data = call_backend()
        return jsonify({"status": "success", "product": data, "circuit": circuit["state"]})
    except Exception as e:
        return jsonify({
            "error": str(e),
            "circuit": circuit["state"],
            "fallback": "Returning cached product data"
        }), 503

@app.route('/health')
def health():
    return jsonify({"status": "healthy", "circuit": circuit["state"]})

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=5000)