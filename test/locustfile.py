import random
import string
from locust import HttpUser, task, between, run_single_user
import uuid

class DataBaseUser(HttpUser):
    wait_time = between(0.5, 1)
    host = "http://localhost:7000"

    writed_keys = []

    def get_sample(self):
        return uuid.uuid4(), ''.join(random.choices(string.ascii_letters + string.digits, k=random.randint(500, 10000)))
    
    @task(1)
    def write(self):
        key, value = self.get_sample()
        self.client.post("/{}".format(key), value, name="write")
        self.writed_keys.append(key)


    @task(10)
    def read(self):
        if len(self.writed_keys) == 0:
            return
        self.client.get("/{}".format(random.choice(self.writed_keys)), name="read")

if __name__ == "__main__":
    run_single_user(DataBaseUser)
