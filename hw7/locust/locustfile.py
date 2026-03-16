import random
from locust import HttpUser, task, between

ORDER_PAYLOAD = {
    "customer_id": 123,
    "items": [
        {"product_id": "prod-1", "quantity": 2, "price": 29.99},
        {"product_id": "prod-2", "quantity": 1, "price": 49.99}
    ]
}

class SyncUser(HttpUser):
    wait_time = between(0.1, 0.5)

    @task
    def place_sync_order(self):
        self.client.post("/orders/sync", json=ORDER_PAYLOAD)


class AsyncUser(HttpUser):
    wait_time = between(0.1, 0.5)

    @task
    def place_async_order(self):
        self.client.post("/orders/async", json=ORDER_PAYLOAD)