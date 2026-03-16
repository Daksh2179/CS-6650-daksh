from locust import task, between
from locust.contrib.fasthttp import FastHttpUser
import random

CATEGORIES = ["Electronics", "Books", "Home", "Sports", "Clothing"]
BRANDS = ["Alpha", "Beta", "Gamma", "Delta"]

class ProductSearchUser(FastHttpUser):
    wait_time = between(0.1, 0.5)

    @task(3)
    def search_category(self):
        term = random.choice(CATEGORIES)
        self.client.get(f"/products/search?q={term}")

    @task(2)
    def search_brand(self):
        term = random.choice(BRANDS)
        self.client.get(f"/products/search?q={term}")

    @task(1)
    def health_check(self):
        self.client.get("/health")