# Game Rules

### Overview
This is a two-player territory control game played on a **10×10 grid**. Players allocate **command points** to cells over multiple rounds in order to control territory and generate additional command points.

Games last **10 rounds**, with **30 seconds per round**, for a total game time of **5 minutes**.

---

### Command Points
Players start the game with **1000 command points**.

Command points represent the resources that players allocate to cells in order to compete for control.

---

### Grid
The game board consists of a **10×10 grid of cells**.

Each cell has one of three states:

- **Neutral** — both players have allocated the same number of command points to the cell
- **Player A Controlled** — Player A has allocated more command points  
- **Player B Controlled** — Player B has allocated more command points  

Control of cells is determined purely by comparing the total command points allocated by each player.

---

### Rounds
The game proceeds in **10 rounds**.

Each round lasts **30 seconds**.

During a round, players may allocate any number of their available command points to any cells on the board.

Rules for allocation:

- Command points placed on a cell **remain there for the entire game**
- Players may distribute command points **across any number of cells**
- Players may allocate **any non-negative amount** of command points to a cell
- Players may allocate command points **incrementally during the round**

At the end of the round, all allocations are finalized.

---

### Cell Control
After allocations are processed:

- If **Player A > Player B** in a cell → Player A controls the cell
- If **Player B > Player A** in a cell → Player B controls the cell
- If **Player A = Player B** → the cell is **neutral**

---

### Command Point Generation
Cells controlled by a player generate additional command points.

If a player **controls a cell**, then **each adjacent cell** produces additional command points in the next round.

Adjacency includes the four cardinal directions:

- up
- down
- left
- right

For every controlled cell:

- Each adjacent cell **spawns 5 new command points for the controlling player at the start of the next round**

This means that over time, the total number of command points in the game may **grow beyond the initial 2000 command points**.

---

### Game End
The game ends after **10 rounds**.

At the end of the final round, territory control is evaluated.

Player controlling the **most cells wins**

---
