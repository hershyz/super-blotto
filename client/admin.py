import os
import sys
import time
import json
import threading
import urllib.request
import urllib.error


# consts
HOSTNAME = ""
PORT = "3001"

# ANSI colors — only the basic 3/4-bit codes, supported on virtually every terminal
RESET = "\033[0m"
BOLD = "\033[1m"
RED = "\033[31m"
GREEN = "\033[32m"
YELLOW = "\033[33m"
CYAN = "\033[36m"


# http helpers
def _post(url, data, headers=None):
    body = json.dumps(data).encode()
    req = urllib.request.Request(url, data=body, headers=headers or {}, method="POST")
    req.add_header("Content-Type", "application/json")
    with urllib.request.urlopen(req) as resp:
        return json.loads(resp.read())

def _get(url, headers=None):
    req = urllib.request.Request(url, headers=headers or {})
    with urllib.request.urlopen(req) as resp:
        return json.loads(resp.read())

def _get_status(url, headers=None):
    req = urllib.request.Request(url, headers=headers or {})
    with urllib.request.urlopen(req) as resp:
        return resp.status


# admin api functions
def api_admin_ping(token):
    return _get_status(f"{HOSTNAME}/adminPing", {"Authorization": token})

def api_admin_status(token):
    return _get(f"{HOSTNAME}/adminStatus", {"Authorization": token})


def clear_screen():
    os.system("cls" if os.name == "nt" else "clear")


def api_start(token):
    return _post(f"{HOSTNAME}/start", {}, {"Authorization": token})

def api_kick(token, username):
    return _post(f"{HOSTNAME}/kick", {"username": username}, {"Authorization": token})

def api_player_stats(token):
    return _get(f"{HOSTNAME}/playerStats", {"Authorization": token})


def phase_color(phase):
    if phase == "lobby":
        return YELLOW
    if phase == "playing":
        return GREEN
    return CYAN


def render_status(status, message=""):
    clear_screen()
    print(RESET, end="", flush=True)
    phase = status.get("phase", "")
    print(f"{BOLD}{'=' * 40}")
    print(f"          ADMIN DASHBOARD")
    print(f"{'=' * 40}{RESET}")
    print(f"  Phase:      {phase_color(phase)}{phase}{RESET}")
    print(f"  Round:      {CYAN}{status.get('round')}{RESET}")
    print(f"  Registered: {CYAN}{status.get('registeredCount')}{RESET}")
    print(f"  In lobby:   {CYAN}{status.get('waitingCount')}{RESET}")
    print(f"{BOLD}{'-' * 40}{RESET}")
    print(f"  Type 'help' for commands")
    print(f"{BOLD}{'=' * 40}{RESET}")
    if message:
        print(f"  {message}{RESET}")


def admin_monitor(token):
    prev_status = None
    last_message = ""
    lock = threading.Lock()

    def poll():
        nonlocal prev_status
        while True:
            try:
                status = api_admin_status(token)
            except (urllib.error.URLError, urllib.error.HTTPError):
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
            print(YELLOW, end="", flush=True)
            cmd = input("> ").strip().lower()
            print(RESET, end="", flush=True)
        except (EOFError, KeyboardInterrupt):
            break

        msg = ""
        if cmd == "help":
            msg = f"{YELLOW}Commands: start, kick <username>, playerstats, help, quit{RESET}"
        elif cmd == "start":
            try:
                resp = api_start(token)
                msg = f"{RED}start: {resp}{RESET}" if "error" in resp else f"{GREEN}Game started!{RESET}"
            except (urllib.error.URLError, urllib.error.HTTPError) as e:
                msg = f"{RED}Error: {e}{RESET}"
        elif cmd.startswith("kick "):
            username = cmd[5:].strip()
            if not username:
                msg = f"{YELLOW}Usage: kick <username>{RESET}"
            else:
                try:
                    resp = api_kick(token, username)
                    msg = f"{RED}kick: {resp}{RESET}" if "error" in resp else f"{GREEN}Kicked {username}{RESET}"
                except (urllib.error.URLError, urllib.error.HTTPError) as e:
                    msg = f"{RED}Error: {e}{RESET}"
        elif cmd == "kick":
            msg = f"{YELLOW}Usage: kick <username>{RESET}"
        elif cmd == "playerstats":
            try:
                resp = api_player_stats(token)
                players = sorted(resp.get("players", []), key=lambda p: p["username"])
                lines = [f"  {p['username']}  {GREEN}{p['wins']}W{RESET}/{RED}{p['losses']}L{RESET}/{p['ties']}T" for p in players]
                msg = f"Players ({resp.get('count', 0)}):\n" + "\n".join(lines) if lines else "No players registered."
            except (urllib.error.URLError, urllib.error.HTTPError) as e:
                msg = f"{RED}Error: {e}{RESET}"
        elif cmd == "quit":
            break
        else:
            msg = f"{YELLOW}Unknown command: {cmd}. Type 'help' for commands.{RESET}"

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

    print(f"{CYAN}Pinging server...{RESET}")
    try:
        status = api_admin_ping(token)
    except (urllib.error.URLError, urllib.error.HTTPError) as e:
        print(f"{RED}Error: could not connect to server: {e}{RESET}")
        sys.exit(1)

    if status != 200:
        print(f"{RED}Admin token not recognized by server.{RESET}")
        sys.exit(1)

    print(f"{GREEN}Admin token verified!{RESET}")

    admin_monitor(token)


if __name__ == "__main__":
    main()
