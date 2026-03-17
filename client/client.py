# imports
import os
import sys
import time
import threading
import requests
from datetime import datetime, timezone


# consts
HOSTNAME = ""
PORT = "3000"

# ANSI colors — basic 3/4-bit codes, supported on virtually every terminal
RESET = "\033[0m"
BOLD = "\033[1m"
RED = "\033[31m"
GREEN = "\033[32m"
YELLOW = "\033[33m"


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

def api_lobby_state(token):
    r = requests.get(f"{HOSTNAME}/lobbyState", headers={"Authorization": token})
    return r.json()


def clear_screen():
    os.system("cls" if os.name == "nt" else "clear")


def render_lobby(players, count, username=""):
    clear_screen()
    print("=" * 40)
    print("           SUPER BLOTTO LOBBY")
    print("=" * 40)
    print(f"  Players waiting: {count}")
    print("-" * 40)
    for p in players:
        if p == username:
            print(f"  > {GREEN}{p}{RESET}")
        else:
            print(f"  > {p}")
    print("-" * 40)
    print("  Waiting for admin to start game...")
    print("=" * 40)


def in_lobby(token, username=""):
    prev_players = None

    while True:
        try:
            resp = api_lobby_state(token)
        except requests.RequestException as e:
            print(f"Error polling lobby: {e}")
            time.sleep(2)
            continue

        phase = resp.get("phase")
        if phase != "lobby":
            clear_screen()
            print("=" * 40)
            print("        GAME IS STARTING!")
            print("=" * 40)
            time.sleep(1)
            return

        players = resp.get("players", [])
        count = resp.get("count", 0)

        if sorted(players) != prev_players:
            prev_players = sorted(players)
            render_lobby(prev_players, count, username)

        time.sleep(1)


def render_game(state, moves_this_round, revealed_board, message=""):
    clear_screen()
    print(RESET, end="", flush=True)

    round_num = state["round"]
    cp = state["command_points"]
    role = state["role"]
    board = state["board"]
    round_end_time = state["round_end_time"]
    phase = state["phase"]

    phase_display = phase.upper() if phase != "playing" else ""
    phase_str = f" | {phase_display}" if phase_display else ""

    print(f"Round {round_num}/10 | CP: {cp}{phase_str} | You are Player {role}")
    print()

    if board is None:
        print("  Waiting for board data...")
        if message:
            print(f"\n  {message}")
        return

    # Column headers
    header = "     "
    for c in range(10):
        header += f" {c:^4}"
    print(header)

    separator = "  +" + "----+" * 10

    for r in range(10):
        print(separator)

        # Top row: opponent CP (show last revealed state during play)
        top = f"{r} |"
        for c in range(10):
            opp_board = revealed_board if revealed_board else board
            opp_cp = opp_board[r][c][1 - role]
            top += f"{RED}{opp_cp:>4}{RESET}|" if opp_cp else "    |"
        print(top)

        # Bottom row: your CP
        bot = "  |"
        for c in range(10):
            my_cp = board[r][c][role]
            bot += f"{GREEN}{my_cp:>4}{RESET}|" if my_cp else "    |"
        print(bot)

    print(separator)
    print()

    if moves_this_round:
        print("Moves this round:")
        move_strs = [f"  ({m[0]},{m[1]}) +{m[2]}cp" for m in moves_this_round]
        print("".join(move_strs))
        print()

    if message:
        print(f"  {message}")

    # Compute timer
    if round_end_time and phase == "playing":
        try:
            end_dt = datetime.fromisoformat(round_end_time.replace("Z", "+00:00"))
            remaining = max(0, int((end_dt - datetime.now(timezone.utc)).total_seconds()))
        except (ValueError, TypeError):
            remaining = 0
        print(f"{remaining}s left in round | Enter move as: row,col,cp")
    else:
        print("Enter move as: row,col,cp")

    # Keep yellow active for user input
    print(YELLOW, end="", flush=True)


def in_game(token):
    state = {
        "round": 0,
        "phase": "playing",
        "round_end_time": None,
        "command_points": 0,
        "role": 0,
        "board": None,
    }
    moves_this_round = []
    revealed_board = None  # opponent board snapshot from last round transition
    last_message = ""
    lock = threading.Lock()
    game_over = threading.Event()

    def poll():
        nonlocal state, moves_this_round, revealed_board
        prev_round = 0
        prev_render_key = None

        while not game_over.is_set():
            try:
                resp = api_state(token)
            except requests.RequestException:
                time.sleep(2)
                continue

            phase = resp.get("phase", "")

            with lock:
                new_round = resp.get("round", 0)
                if new_round != prev_round:
                    # Snapshot the board at round transition — this is the
                    # resolved state that includes opponent moves from last round
                    revealed_board = resp.get("board")
                    moves_this_round = []
                    prev_round = new_round

                state = {
                    "round": new_round,
                    "phase": phase,
                    "round_end_time": resp.get("roundEndTime", ""),
                    "command_points": resp.get("commandPoints", 0),
                    "role": resp.get("role", 0),
                    "board": resp.get("board"),
                }

                if phase in ("lobby", "finished"):
                    render_game(state, moves_this_round, None, last_message)
                    game_over.set()
                    return

                # Only re-render on changes visible to this player
                render_key = (new_round, phase, state["command_points"])
                if render_key != prev_render_key:
                    prev_render_key = render_key
                    render_game(state, moves_this_round, revealed_board, last_message)

            time.sleep(1)

    t = threading.Thread(target=poll, daemon=True)
    t.start()

    while not game_over.is_set():
        try:
            print(YELLOW, end="", flush=True)
            line = input().strip()
            print(RESET, end="", flush=True)
        except (EOFError, KeyboardInterrupt):
            game_over.set()
            break

        if game_over.is_set():
            break

        if not line:
            continue

        parts = line.split(",")
        if len(parts) != 3:
            with lock:
                last_message = "Invalid format. Use: row,col,cp"
                render_game(state, moves_this_round, revealed_board, last_message)
            continue

        try:
            row, col, cp = int(parts[0]), int(parts[1]), int(parts[2])
        except ValueError:
            with lock:
                last_message = "Invalid numbers. Use: row,col,cp"
                render_game(state, moves_this_round, revealed_board, last_message)
            continue

        if cp <= 0:
            with lock:
                last_message = "CP must be positive."
                render_game(state, moves_this_round, revealed_board, last_message)
            continue

        with lock:
            current_round = state["round"]

        try:
            resp = api_move(token, current_round, row, col, cp)
        except requests.RequestException as e:
            with lock:
                last_message = f"Network error: {e}"
                render_game(state, moves_this_round, revealed_board, last_message)
            continue

        if "error" in resp:
            with lock:
                last_message = f"Move failed: {resp['error']}"
                render_game(state, moves_this_round, revealed_board, last_message)
        else:
            with lock:
                moves_this_round.append((row, col, cp))
                last_message = f"Placed {cp}cp at ({row},{col})"
                render_game(state, moves_this_round, revealed_board, last_message)

    clear_screen()
    print("=" * 40)
    print("        GAME OVER!")
    print("=" * 40)
    time.sleep(2)


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

    while True:
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

        in_lobby(token, username)
        in_game(token)

        print("Returning to lobby...")
        time.sleep(1)


if __name__ == "__main__":
    main()
