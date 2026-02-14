from locust import HttpUser, task, between
import random
import json

class ProductAPIUser(HttpUser):
    wait_time = between(1, 3)
    host = "http://54.188.109.70:8080"
    
    def on_start(self):
        """Create initial products when user starts"""
        self.product_ids = []
        for i in range(1, 6):
            response = self.client.post(
                f"/products/{i}/details",
                json={
                    "product_id": i,
                    "sku": f"SKU-{i}-{random.randint(1000, 9999)}",
                    "manufacturer": f"Manufacturer {i}",
                    "category_id": random.randint(1, 100),
                    "weight": random.randint(100, 5000),
                    "some_other_id": random.randint(1, 1000)
                },
                name="/products/[id]/details POST"
            )
            if response.status_code == 204:
                self.product_ids.append(i)
    
    @task(5)  # 5x weight - reads are most common
    def get_product(self):
        """GET /products/{productId}"""
        if self.product_ids:
            product_id = random.choice(self.product_ids)
            self.client.get(
                f"/products/{product_id}",
                name="/products/[id] GET"
            )
    
    @task(1)  # 1x weight - writes less common
    def add_product_details(self):
        """POST /products/{productId}/details"""
        product_id = random.randint(1, 100)
        self.client.post(
            f"/products/{product_id}/details",
            json={
                "product_id": product_id,
                "sku": f"SKU-{random.randint(10000, 99999)}",
                "manufacturer": f"Manufacturer {random.randint(1, 50)}",
                "category_id": random.randint(1, 100),
                "weight": random.randint(100, 5000),
                "some_other_id": random.randint(1, 1000)
            },
            name="/products/[id]/details POST"
        )