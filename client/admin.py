import sys
import requests


# consts
HOSTNAME = ""
PORT = "3000"


# admin api functions
def api_admin_ping(token):
    r = requests.get(f"{HOSTNAME}/adminPing", headers={"Authorization": token})
    return r.status_code


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


if __name__ == "__main__":
    main()
