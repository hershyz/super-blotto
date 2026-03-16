# imports
import sys
import time
import requests


# consts
HOSTNAME = ""
PORT = "3000"


# client-facing api functions
def api_register(username):
    r = requests.post(f"{HOSTNAME}/register", json={"username": username})
    return r.json()

def api_join(token):
    r = requests.post(f"{HOSTNAME}/join", json={}, headers={"Authorization": token})
    return r.json()

def api_leave(token):
    r = requests.post(f"{HOSTNAME}/leave", json={}, headers={"Authorization": token})
    return r.json()

def api_move(token, round_num, row, col, cp):
    r = requests.post(f"{HOSTNAME}/move", json={
        "round": round_num, "row": row, "col": col, "commandPoints": cp
    }, headers={"Authorization": token})
    return r.json()

def api_state(token):
    r = requests.get(f"{HOSTNAME}/state", headers={"Authorization": token})
    return r.json()

def api_start(token):
    r = requests.post(f"{HOSTNAME}/start", json={}, headers={"Authorization": token})
    return r.json()

def api_lobby(token):
    r = requests.post(f"{HOSTNAME}/lobby", json={}, headers={"Authorization": token})
    return r.json()


# client flow
def main():
    host = input("Enter server host (e.g. http://localhost): ").strip()
    if not host:
        print("No host provided, exiting.")
        sys.exit(1)

    global HOSTNAME
    HOSTNAME = f"{host}:{PORT}"

    username = input("Enter username: ").strip()
    if not username:
        print("No username provided, exiting.")
        sys.exit(1)

    print(f"Registering as '{username}'...")
    try:
        resp = api_register(username)
    except requests.RequestException as e:
        print(f"Error: could not connect to server: {e}")
        sys.exit(1)

    if "error" in resp:
        print(f"Registration failed: {resp['error']}")
        sys.exit(1)

    token = resp.get("token")
    if not token:
        print("Registration failed: no token in response")
        sys.exit(1)

    print(f"Registration successful! Your token: {token}")

    time.sleep(1)

    print("Joining lobby...")
    try:
        resp = api_join(token)
    except requests.RequestException as e:
        print(f"Error: could not connect to server: {e}")
        sys.exit(1)

    if "error" in resp:
        print(f"Failed to join lobby: {resp['error']}")
        sys.exit(1)

    print("Joined lobby!")


if __name__ == "__main__":
    main()
