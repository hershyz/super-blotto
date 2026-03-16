import requests
import time
from datetime import datetime, timezone

BASE_URL = "http://localhost:8080"
ADMIN_TOKEN = "AdminToken"

# API FUNCTIONS

def api_register(username):
    r = requests.post(f"{BASE_URL}/register", json={"username": username})
    return r.json()

def api_join(token):
    r = requests.post(f"{BASE_URL}/join", json={}, headers={"Authorization": token})
    return r.json()

def api_leave(token):
    r = requests.post(f"{BASE_URL}/leave", json={}, headers={"Authorization": token})
    return r.json()

def api_move(token, round_num, row, col, cp):
    r = requests.post(f"{BASE_URL}/move", json={
        "round": round_num, "row": row, "col": col, "commandPoints": cp
    }, headers={"Authorization": token})
    return r.json()

def api_state(token):
    r = requests.get(f"{BASE_URL}/state", headers={"Authorization": token})
    return r.json()

def api_start(token):
    r = requests.post(f"{BASE_URL}/start", json={}, headers={"Authorization": token})
    return r.json()

def api_lobby(token):
    r = requests.post(f"{BASE_URL}/lobby", json={}, headers={"Authorization": token})
    return r.json()

# DISPLAY FUNCTIONS

def time_left(round_end_str, phase):
    if phase != "playing" or not round_end_str:
        return ""
    try:
        end = datetime.fromisoformat(round_end_str.replace("Z", "+00:00"))
        secs = max(0, int((end - datetime.now(timezone.utc)).total_seconds()))
        return f" | {secs}s left"
    except Exception:
        return ""

def render_board(board, role):
    # board[row][col] = [p0_points, p1_points]
    # A = player 0, B = player 1
    print("\n    " + "  ".join(f"{c}" for c in range(10)))
    print("   +" + "--" * 10 + "-+")
    for row in range(10):
        cells = []
        for col in range(10):
            p0, p1 = board[row][col][0], board[row][col][1]
            if p0 == p1:
                cells.append(".")
            elif p0 > p1:
                cells.append("A")
            else:
                cells.append("B")
        print(f" {row:2}| " + "  ".join(cells) + " |")
    print("   +" + "--" * 10 + "-+")
    you = "A" if role == 0 else "B"
    them = "B" if role == 0 else "A"
    print(f"    You = {you}   Opponent = {them}")

def show_state(state):
    phase = state.get("phase", "?")
    round_num = state.get("round", 0)
    role = state.get("role", -1)
    cp = state.get("commandPoints", 0)
    board = state.get("board")
    tl = time_left(state.get("roundEndTime"), phase)

    print(f"\n=== Round {round_num}/10  |  Phase: {phase}{tl} ===")
    if role >= 0:
        print(f"You are Player {'A' if role == 0 else 'B'}  |  Command points: {cp}")
    if board:
        render_board(board, role)


def admin_flow():
    while True:
        print("\n--- Admin ---")
        print("1. Start Game")
        print("2. Lobby")
        print("3. Exit")
        choice = input("> ").strip()
        if choice == "1":
            res = api_start(ADMIN_TOKEN)
            if "error" in res:
                print(f"Error: {res['error']}")
            else:
                print("Game started!")
        elif choice == "2":
            res = api_lobby(ADMIN_TOKEN)
            if "error" in res:
                print(f"Error: {res['error']}")
            else:
                print("Reset to lobby.")
        elif choice == "3":
            return False # handle
        else:
            print("Invalid option.")


def player_in_game_flow(token):
    while True:
        print("\n--- In Game ---")
        print("1. View State")
        print("2. Move")
        print("3. Leave")
        choice = input("> ").strip()
        if choice == "1":
            state = api_state(token)
            if "error" in state:
                print(f"Error: {state['error']}")
            else:
                show_state(state)
        elif choice == "2":
            state = api_state(token)
            if "error" in state:
                print(f"Error: {state['error']}")
                continue
            if state["phase"] != "playing":
                print(f"Can't move right now (phase: {state['phase']})")
                continue
            round_num = state["round"]
            try:
                row = int(input("Row (0-9): ").strip())
                col = int(input("Col (0-9): ").strip())
                cp  = int(input("Command points: ").strip())
            except ValueError:
                print("Invalid input.")
                continue
            res = api_move(token, round_num, row, col, cp)
            if "error" in res:
                print(f"Error: {res['error']}")
            else:
                print(f"Placed {cp} CP at ({row}, {col})")
        elif choice == "3":
            res = api_leave(token)
            if "error" in res:
                print(f"Error: {res['error']}")
            else:
                print("Left the game.")
            break
        else:
            print("Invalid option.")


def player_flow():
    token = None
    while True:
        print("\n--- Player ---")
        print("1. Register")
        print("2. Join Game")
        print("3. Exit")
        choice = input("> ").strip()
        if choice == "1":
            username = input("Username: ").strip()
            res = api_register(username)
            if "error" in res:
                print(f"Error: {res['error']}")
            else:
                token = res["token"]
                print(f"Registered! Token: {token}")
        elif choice == "2":
            if not token:
                token = input("Enter your token: ").strip()
            res = api_join(token)
            if "error" in res:
                print(f"Error: {res['error']}")
            else:
                print("Joined! Entering game menu...")
                player_in_game_flow(token)
        elif choice == "3":
            break
        else:
            print("Invalid option.")


# MAIN

print("=== Super Blotto ===")
print("1. Admin")
print("2. Player")
choice = input("> ").strip()
if choice == "1":
    admin_flow()
elif choice == "2":
    player_flow()
else:
    print("Invalid option.")
    
    
