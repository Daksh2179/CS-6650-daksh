from locust import HttpUser, task, between

class ProductUser(HttpUser):
    wait_time = between(0.5, 1)
    host = "http://localhost:5000"

    @task
    def get_product(self):
        self.client.get("/product")