from flask import Flask, jsonify
import requests

app = Flask(__name__)

BACKEND_URL = "http://backend:5001"

@app.route('/product')
def get_product():
    try:
        response = requests.get(f"{BACKEND_URL}/data", timeout=5)
        if response.status_code == 200:
            return jsonify({"status": "success", "product": response.json()})
        return jsonify({"error": "Backend error"}), 500
    except Exception as e:
        return jsonify({"error": str(e)}), 500

@app.route('/health')
def health():
    return jsonify({"status": "healthy"})

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=5000)