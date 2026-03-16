import os
import sys
import time
import threading
import requests


# consts
HOSTNAME = ""
PORT = "3000"


# admin api functions
def api_admin_ping(token):
    r = requests.get(f"{HOSTNAME}/adminPing", headers={"Authorization": token})
    return r.status_code

def api_admin_status(token):
    r = requests.get(f"{HOSTNAME}/adminStatus", headers={"Authorization": token})
    return r.json()


def clear_screen():
    os.system("cls" if os.name == "nt" else "clear")


def api_start(token):
    r = requests.post(f"{HOSTNAME}/start", json={}, headers={"Authorization": token})
    return r.json()


def render_status(status, message=""):
    clear_screen()
    print("=" * 40)
    print("          ADMIN DASHBOARD")
    print("=" * 40)
    print(f"  Phase:      {status.get('phase')}")
    print(f"  Round:      {status.get('round')}")
    print(f"  Registered: {status.get('registeredCount')}")
    print(f"  In lobby:   {status.get('waitingCount')}")
    print("-" * 40)
    print("  Commands: start, quit")
    print("=" * 40)
    if message:
        print(f"  {message}")


def admin_monitor(token):
    prev_status = None
    last_message = ""
    lock = threading.Lock()

    def poll():
        nonlocal prev_status
        while True:
            try:
                status = api_admin_status(token)
            except requests.RequestException:
                time.sleep(2)
                continue

            with lock:
                if status != prev_status:
                    prev_status = status
                    render_status(status, last_message)

            time.sleep(1)

    t = threading.Thread(target=poll, daemon=True)
    t.start()

    while True:
        try:
            cmd = input().strip().lower()
        except (EOFError, KeyboardInterrupt):
            break

        msg = ""
        if cmd == "start":
            try:
                resp = api_start(token)
                msg = f"start: {resp}" if "error" in resp else "Game started!"
            except requests.RequestException as e:
                msg = f"Error: {e}"
        elif cmd == "quit":
            break
        else:
            msg = f"Unknown command: {cmd}"

        with lock:
            last_message = msg
            if prev_status:
                render_status(prev_status, last_message)


# admin flow
def main():
    host = input("Enter server host (e.g. http://localhost): ").strip()
    if not host:
        print("No host provided, exiting.")
        sys.exit(1)

    global HOSTNAME
    HOSTNAME = f"{host}:{PORT}"

    token = input("Enter admin token: ").strip()
    if not token:
        print("No token provided, exiting.")
        sys.exit(1)

    print("Pinging server...")
    try:
        status = api_admin_ping(token)
    except requests.RequestException as e:
        print(f"Error: could not connect to server: {e}")
        sys.exit(1)

    if status != 200:
        print("Admin token not recognized by server.")
        sys.exit(1)

    print("Admin token verified!")

    admin_monitor(token)


if __name__ == "__main__":
    main()
