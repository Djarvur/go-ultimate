// Command game is a minimal Ebitengine 2D game.
//
// Template: game. Layout: flat at root while tiny (single main.go); grow into
// cmd/ + internal/ (scenes, assets, systems) as complexity rises. See
// go-ultimate/references/project-layouts.md § Script and § CLI.
package main

import (
	"log"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

// Game implements ebiten.Game. Add game state as fields on this struct.
type Game struct{}

// Update advances the game state by one tick. Return an error to exit the loop.
func (g *Game) Update() error {
	return nil
}

// Draw renders the current frame to screen.
func (g *Game) Draw(screen *ebiten.Image) {
	ebitenutil.DebugPrint(screen, "Hello, World!")
}

// Layout returns the logical screen size given the window size.
func (g *Game) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return 320, 240
}

func main() {
	ebiten.SetWindowSize(640, 480)
	ebiten.SetWindowTitle("Hello, World!")
	if err := ebiten.RunGame(&Game{}); err != nil {
		log.Fatalf("game: %v", err)
	}
}
