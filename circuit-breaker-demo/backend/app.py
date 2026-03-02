from flask import Flask, jsonify
import time

app = Flask(__name__)

# Simulate three phases:
# Phase 1 (0-60s): healthy
# Phase 2 (60-120s): complete failure
# Phase 3 (120s+): recovered

start_time = time.time()

@app.route('/data')
def get_data():
    elapsed = time.time() - start_time
    
    if elapsed < 60:
        # Phase 1: healthy
        return jsonify({"status": "ok", "data": "product info", "phase": "healthy"})
    elif elapsed < 120:
        # Phase 2: complete failure
        time.sleep(0.1)
        return jsonify({"error": "Backend crashed"}), 500
    else:
        # Phase 3: recovered
        return jsonify({"status": "ok", "data": "product info", "phase": "recovered"})

@app.route('/health')
def health():
    return jsonify({"status": "healthy"})

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=5001)
    