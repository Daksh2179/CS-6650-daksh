from locust import HttpUser, task, between

class AlbumUser(HttpUser):
    wait_time = between(1, 2)
    
    @task(3)  # GET task with weight 3 (3:1 ratio)
    def get_albums(self):
        self.client.get("/albums")
    
    @task(1)  # POST task with weight 1
    def post_album(self):
        self.client.post("/albums", json={
            "id": "4",
            "title": "Test Album",
            "artist": "Test Artist",
            "price": 29.99
        })