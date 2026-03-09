package main

type CellData struct {
	Points [2]int
}
 
type Game struct {
	ID 						int
	Players				[2]*Player
	CommandPoints	[2]int
	Board 				[GridHeight][GridWidth]CellData
}

// All functions are called assuming the caller holds the gs.mu lock

func (g *Game) generatePoints(row, col int, role PlayerRole) {
	for _, d := range dirs {
		nr := row + d[0]
		nc := col + d[1]

		if nr < 0 || nr >= GridHeight ||
		nc < 0 || nc >= GridWidth {
			continue
		}

		g.CommandPoints[role] += 5
	}
}

func (g *Game) endRound() {
	for row := 0; row < GridHeight; row++ {
		for col := 0; col < GridWidth; col++ {
			points := g.Board[row][col].Points

			if points[Player0] > points[Player1] {
				g.generatePoints(row, col, Player0)
			} else if points[Player0] < points[Player1] {
				g.generatePoints(row, col, Player1)
			} else {
				continue
			}
		}
	}
}

func (g *Game) endGame() {
	p0Controlled := 0
	p1Controlled := 0
	for row := 0; row < GridHeight; row++ {
		for col := 0; col < GridWidth; col++ {
			points := g.Board[row][col].Points

			if points[Player0] > points[Player1] {
				p0Controlled++
			} else if points[Player0] < points[Player1] {
				p1Controlled++
			} else {
				continue
			}
		}
	}

	if p0Controlled > p1Controlled {
		g.Players[Player0].Wins++
		g.Players[Player1].Losses++
	} else if p0Controlled < p1Controlled {
		g.Players[Player0].Losses++
		g.Players[Player1].Wins++
	} else {
		g.Players[Player0].Ties++
		g.Players[Player1].Ties++
	}
}

// Row and Col must be validated before calling move
func (g *Game) move(row, col, reqCommandPoints int, role PlayerRole) (error) {
	if g.CommandPoints[role] < reqCommandPoints { return ErrInsufficientCommandPoints }

	g.CommandPoints[role] -= reqCommandPoints
	g.Board[row][col].Points[role] += reqCommandPoints
	return nil
}
