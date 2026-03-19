# Admin / Deployment / Server Instructions

## Running the Game Server

```bash
cd server
ADMIN_TOKEN=[token] go run .
```

> **Note:** Port 3001 must be exposed on the server (update the security group in AWS) and all intercepting layers / reverse proxies (e.g., nginx) must be shut down.

## Running the Admin Console

```bash
cd client
python3 admin.py
```

You will be prompted for the server host and admin token.

### Available Commands

| Command | Description |
|---|---|
| `help` | Display available commands |
| `start` | Start the game |
| `kick <username>` | Kick a player from the game |
| `playerstats` | Display player statistics (wins/losses/ties) |
| `quit` | Exit the admin dashboard |

## Game Lifecycle

- The game will **not start with an odd number of players**. If there is an odd number in the lobby, you need to join yourself to make it even.
- Players cannot join (hit the `/register` endpoint) while the game is running (e.g., 1v1s are active).
- After a game instance ends, the game server returns to a **lobby phase**. The admin must manually exit the lobby by using the `start` command to begin the next round.
- When this lobby transition happens, the game server **persists the leaderboard state** to a local JSON file.

