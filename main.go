package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image/color"
	"log"
	"math"
	"math/rand"
	"os"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

const (
	cellSize   = 20
	gridW      = 32
	gridH      = 24
	minSpeed   = 8
	maxSpeed   = 16
	sampleRate = 44100
	saveFile   = "snake_highscore.json"
)

var (
	bgColor    = color.RGBA{24, 24, 28, 255}
	gridColor  = color.RGBA{40, 40, 48, 255}
	headColor  = color.RGBA{80, 220, 120, 255}
	bodyColor  = color.RGBA{60, 180, 100, 255}
	foodColor  = color.RGBA{230, 70, 70, 255}
)

type Point struct{ X, Y int }

type Game struct {
	snake          []Point
	dir            Point
	nextDir        Point
	grow           int
	food           Point
	rng            *rand.Rand
	frame          int
	speed          int
	score          int
	highScore      int
	gameOver       bool
	paused         bool
	inTitle        bool
	combo          int
	comboTimer     int
	foodPulse      float64
	scaleFactor    float64 // For dynamic scaling
	isFullscreen   bool    // Track maximized/full-screen state

	audioCtx        *audio.Context
	eatPlayer       *audio.Player
	comboPlayer     *audio.Player
	gameOverPlayer  *audio.Player
	bgLoop          *audio.InfiniteLoop
	bgPlayer        *audio.Player
}

func NewGame() *Game {
	g := &Game{rng: rand.New(rand.NewSource(time.Now().UnixNano()))}
	g.loadHighScore()

	g.audioCtx = audio.NewContext(sampleRate)
	g.eatPlayer = newBeepPlayer(g.audioCtx, 880, 0.1)
	g.comboPlayer = newBeepPlayer(g.audioCtx, 1100, 0.15)
	g.gameOverPlayer = newBeepPlayer(g.audioCtx, 220, 0.4)
	g.bgLoop, g.bgPlayer = newBackgroundLoop(g.audioCtx)
	g.bgPlayer.Play()

	g.inTitle = true
	g.speed = 12
	g.scaleFactor = 1.0
	g.isFullscreen = false
	return g
}

func newBeepPlayer(ctx *audio.Context, freq float64, durSec float64) *audio.Player {
	n := int(float64(sampleRate) * durSec)
	buf := make([]byte, n*4)
	for i := 0; i < n; i++ {
		t := float64(i) / sampleRate
		v := int16(math.Sin(2*math.Pi*freq*t) * 4000 * math.Pow(math.E, -3*t))
		for ch := 0; ch < 2; ch++ {
			idx := i*4 + ch*2
			buf[idx] = byte(v)
			buf[idx+1] = byte(v >> 8)
		}
	}
	p := ctx.NewPlayerFromBytes(buf)
	return p
}

func newBackgroundLoop(ctx *audio.Context) (*audio.InfiniteLoop, *audio.Player) {
	notes := []float64{261.63, 329.63, 392.00, 523.25}
	durSec := 0.25
	totalSamples := int(float64(sampleRate) * durSec * float64(len(notes)))
	buf := make([]byte, totalSamples*4)
	idx := 0
	for _, freq := range notes {
		for i := 0; i < int(float64(sampleRate)*durSec); i++ {
			t := float64(i) / sampleRate
			v := int16(math.Sin(2*math.Pi*freq*t) * 2000 * math.Pow(math.E, -2*t))
			for ch := 0; ch < 2; ch++ {
				buf[idx] = byte(v)
				buf[idx+1] = byte(v >> 8)
				idx += 2
			}
		}
	}
	src := bytes.NewReader(buf)
	loop := audio.NewInfiniteLoop(src, int64(len(buf)))
	player, err := ctx.NewPlayer(loop)
	if err != nil {
		log.Fatal(err)
	}
	return loop, player
}

func (g *Game) reset() {
	midX, midY := gridW/2, gridH/2
	g.snake = []Point{{midX, midY}, {midX-1, midY}, {midX-2, midY}}
	g.dir = Point{1, 0}
	g.nextDir = g.dir
	g.grow = 0
	g.frame = 0
	g.score = 0
	g.combo = 0
	g.comboTimer = 0
	g.gameOver = false
	g.paused = false
	g.inTitle = false
	g.foodPulse = 0
	g.placeFood()
	g.bgPlayer.Rewind()
	g.bgPlayer.Play()
}

func (g *Game) loadHighScore() {
	data, err := os.ReadFile(saveFile)
	if err != nil {
		g.highScore = 0
		return
	}
	_ = json.Unmarshal(data, &g.highScore)
}

func (g *Game) saveHighScore() {
	data, _ := json.Marshal(g.highScore)
	_ = os.WriteFile(saveFile, data, 0644)
}

func (g *Game) placeFood() {
	for {
		f := Point{g.rng.Intn(gridW), g.rng.Intn(gridH)}
		occupied := false
		for _, s := range g.snake {
			if s == f {
				occupied = true
				break
			}
		}
		if !occupied {
			g.food = f
			return
		}
	}
}

func (g *Game) Update() error {
	// Toggle full-screen/maximized with F key
	if inpututil.IsKeyJustPressed(ebiten.KeyF) {
		g.isFullscreen = !g.isFullscreen
		if g.isFullscreen {
			ebiten.MaximizeWindow()
		} else {
			ebiten.RestoreWindow()
			ebiten.SetWindowSize(1280, 720)
		}
	}

	// Exit full-screen/maximized with Esc key
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		if g.isFullscreen {
			g.isFullscreen = false
			ebiten.RestoreWindow()
			ebiten.SetWindowSize(1280, 720)
		}
	}

	if g.inTitle {
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeySpace) {
			g.reset()
		}
		return nil
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyP) {
		g.paused = !g.paused
		if g.paused {
			g.bgPlayer.Pause()
		} else {
			g.bgPlayer.Play()
		}
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyEqual) {
		if g.speed > minSpeed {
			g.speed--
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyMinus) {
		if g.speed < maxSpeed {
			g.speed++
		}
	}

	if g.gameOver {
		g.bgPlayer.Pause()
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyR) {
			if g.score > g.highScore {
				g.highScore = g.score
				g.saveHighScore()
			}
			g.reset()
		}
		return nil
	}

	dir := g.dir
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) || inpututil.IsKeyJustPressed(ebiten.KeyW) {
		if dir.Y != 1 { g.nextDir = Point{0, -1} }
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) || inpututil.IsKeyJustPressed(ebiten.KeyS) {
		if dir.Y != -1 { g.nextDir = Point{0, 1} }
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) || inpututil.IsKeyJustPressed(ebiten.KeyA) {
		if dir.X != 1 { g.nextDir = Point{-1, 0} }
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) || inpututil.IsKeyJustPressed(ebiten.KeyD) {
		if dir.X != -1 { g.nextDir = Point{1, 0} }
	}

	if g.paused {
		return nil
	}

	g.frame++
	g.foodPulse += 0.05
	if g.frame%g.speed != 0 {
		return nil
	}

	g.dir = g.nextDir
	head := g.snake[0]
	newHead := Point{(head.X + g.dir.X + gridW) % gridW, (head.Y + g.dir.Y + gridH) % gridH}

	for _, s := range g.snake {
		if s == newHead {
			g.gameOver = true
			g.gameOverPlayer.Rewind()
			g.gameOverPlayer.Play()
			if g.score > g.highScore {
				g.highScore = g.score
				g.saveHighScore()
			}
			return nil
		}
	}

	g.snake = append([]Point{newHead}, g.snake...)
	if newHead == g.food {
		g.grow += 2
		g.combo++
		g.comboTimer = 60
		g.score += 1 + g.combo/2
		if g.combo > 1 {
			g.comboPlayer.Rewind()
			g.comboPlayer.Play()
		} else {
			g.eatPlayer.Rewind()
			g.eatPlayer.Play()
		}
		g.placeFood()
	} else {
		g.comboTimer--
		if g.comboTimer <= 0 {
			g.combo = 0
		}
	}
	if g.grow > 0 {
		g.grow--
	} else {
		g.snake = g.snake[:len(g.snake)-1]
	}

	return nil
}

func drawCell(x, y int, c color.Color, screen *ebiten.Image, scale float64, g *Game) {
	size := float64(cellSize) * scale * g.scaleFactor
	offset := float64(cellSize) * (1-scale) / 2
	ebitenutil.DrawRect(screen, (float64(x*cellSize)+offset)*g.scaleFactor, (float64(y*cellSize)+offset)*g.scaleFactor, size, size, c)
}

func (g *Game) Draw(screen *ebiten.Image) {
	// Solid background
	screen.Fill(bgColor)

	// Grid lines
	for x := 0; x < gridW; x++ {
		ebitenutil.DrawRect(screen, float64(x*cellSize)*g.scaleFactor, 0, 1*g.scaleFactor, float64(gridH*cellSize)*g.scaleFactor, gridColor)
	}
	for y := 0; y < gridH; y++ {
		ebitenutil.DrawRect(screen, 0, float64(y*cellSize)*g.scaleFactor, float64(gridW*cellSize)*g.scaleFactor, 1*g.scaleFactor, gridColor)
	}

	screenWidth := float64(gridW * cellSize)
	screenHeight := float64(gridH * cellSize)

	if g.inTitle {
		// Center title screen text both horizontally and vertically
		lines := []string{
			"Snake!",
			"Eat food to grow and score points!",
			"Arrow Keys/WASD: Move, P: Pause, F: Maximize, Esc: Restore",
			"Press Enter or Space to start!",
			"Developed by Sarwar Hossain",
		}
		lineHeight := 20.0 * g.scaleFactor
		totalHeight := float64(len(lines)) * lineHeight
		startY := (screenHeight - totalHeight) / 2
		for i, line := range lines {
			approxWidth := float64(len(line)) * 8
			x := (screenWidth - approxWidth) / 2
			y := startY + float64(i)*lineHeight
			ebitenutil.DebugPrintAt(screen, line, int(x*g.scaleFactor), int(y*g.scaleFactor))
		}
		return
	}

	// Subtle pulsing food effect
	pulse := 0.9 + 0.1*math.Sin(g.foodPulse)
	foodC := foodColor
	if g.combo > 0 {
		foodC = color.RGBA{255, 165, 0, 255}
	}
	drawCell(g.food.X, g.food.Y, foodC, screen, pulse, g)

	// Snake with uniform body color
	for i, s := range g.snake {
		if i == 0 {
			drawCell(s.X, s.Y, headColor, screen, 1.0, g)
		} else {
			drawCell(s.X, s.Y, bodyColor, screen, 0.9, g)
		}
	}

	// HUD in top-left with padding
	lines := []string{
		fmt.Sprintf("Score: %d | High Score: %d | Speed: %d", g.score, g.highScore, g.speed),
		"Controls: Arrow/WASD (Move), P (Pause), F (Maximize), Esc (Restore)",
	}
	if g.paused {
		lines = append(lines, "Paused - Press P to Resume")
	} else if g.gameOver {
		lines = append(lines, fmt.Sprintf("Game Over! Score: %d - Press Enter/R to Retry", g.score))
	}
	if g.combo > 0 {
		lines = append(lines, fmt.Sprintf("Combo: x%d (Bonus: +%d)", g.combo, g.combo/2))
	}

	// Draw HUD in top-left with 10-pixel padding (scaled)
	padding := 10.0 * g.scaleFactor
	lineHeight := 20.0 * g.scaleFactor
	for i, line := range lines {
		ebitenutil.DebugPrintAt(screen, line, int(padding), int(padding+float64(i)*lineHeight))
	}
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	// Update isFullscreen based on window state
	g.isFullscreen = ebiten.IsWindowMaximized()
	// Calculate scale factor
	scaleX := float64(outsideWidth) / float64(gridW*cellSize)
	scaleY := float64(outsideHeight) / float64(gridH*cellSize)
	g.scaleFactor = math.Min(scaleX, scaleY)
	return int(float64(gridW*cellSize) * g.scaleFactor), int(float64(gridH*cellSize) * g.scaleFactor)
}

func main() {
	ebiten.SetWindowSize(1280, 720)
	ebiten.SetWindowTitle("Snake — Go + Ebiten")
	ebiten.SetWindowResizable(true)
	if err := ebiten.RunGame(NewGame()); err != nil {
		log.Fatal(err)
	}
}