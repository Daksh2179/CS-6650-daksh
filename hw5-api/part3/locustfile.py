from locust import task, between
from locust.contrib.fasthttp import FastHttpUser
import random

SEARCH_TERMS = ["Electronics", "Alpha", "Books", "Home", "Beta", "Sports", "Gamma", "Clothing"]

class ProductSearchUser(FastHttpUser):
    wait_time = between(0.1, 0.5)  # minimal wait = maximum CPU pressure

    @task(3)
    def search_category(self):
        term = random.choice(["Electronics", "Books", "Home", "Sports", "Clothing"])
        self.client.get(f"/products/search?q={term}")

    @task(2)
    def search_brand(self):
        term = random.choice(["Alpha", "Beta", "Gamma", "Delta"])
        self.client.get(f"/products/search?q={term}")

    @task(1)
    def health_check(self):
        self.client.get("/health")