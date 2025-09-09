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
	"github.com/hajimehoshi/ebiten/v2/text"

	"golang.org/x/image/font/basicfont"
)

const (
	baseCellSize = 20
	baseGridW    = 32
	baseGridH    = 24
	minSpeed     = 4
	maxSpeed     = 20
	sampleRate   = 44100
	saveFile     = "snake_enhanced.json"
)

// ==================== TYPES ====================

type Point struct{ X, Y int }
type Vector2 struct{ X, Y float64 }

type Particle struct {
	pos      Vector2
	vel      Vector2
	life     float64
	maxLife  float64
	color    color.RGBA
	size     float64
	rotation float64
	rotVel   float64
}

type Meteor struct {
	pos    Vector2
	vel    Vector2
	size   float64
	color  color.RGBA
	trail  []Vector2
	life   float64
	glow   float64
}

type PowerUp struct {
	pos      Point
	type_    int // 0: bonus points, 1: speed boost, 2: invulnerability
	timer    int
	active   bool
	pulse    float64
	sparkles []Particle
}

type GameData struct {
	HighScore    int   `json:"high_score"`
	TotalGames   int   `json:"total_games"`
	TotalScore   int   `json:"total_score"`
	BestCombo    int   `json:"best_combo"`
	PlayTime     int64 `json:"play_time_seconds"`
}

type GameState int

const (
	StateTitleScreen GameState = iota
	StateMenu
	StatePlaying
	StatePaused
	StateGameOver
)

type Renderer struct {
	game           *Game
	backgroundGrid [][]BackgroundCell
	starField      []Star
	meteors        []Meteor
	time           float64
}

type BackgroundCell struct {
	intensity  float64
	phase      float64
	colorShift float64
}

type Star struct {
	pos       Vector2
	brightness float64
	twinkle    float64
	speed      float64
	size       float64
}

type Game struct {
	// Core game state
	snake          []Point
	dir            Point
	nextDir        Point
	grow           int
	food           Point
	powerUp        PowerUp
	particles      []Particle
	rng            *rand.Rand
	frame          int
	speed          int
	baseSpeed      int
	score          int
	gameData       GameData
	combo          int
	maxCombo       int
	comboTimer     int
	gameStartTime  time.Time

	// Game state management
	state         GameState
	menuOption    int
	isFullscreen  bool

	// Visual effects
	foodPulse      float64
	scaleFactor    float64
	screenWidth    int
	screenHeight   int
	gridW          int
	gridH          int
	cellSize       int
	speedBoostTime int
	slowMotionTime int
	invulnerable   int
	shakeIntensity float64
	trailOpacity   []float64
	headPulse      float64

	// Audio system
	audioCtx       *audio.Context
	eatPlayer      *audio.Player
	comboPlayer    *audio.Player
	powerUpPlayer  *audio.Player
	gameOverPlayer *audio.Player
	bgLoop         *audio.InfiniteLoop
	bgPlayer       *audio.Player

	// Renderer
	renderer *Renderer
}

// ==================== GREEN/BLACK/RED COLOR PALETTE ====================

var (
	// Base colors - Green/Black/Red theme
	bgColor     = color.RGBA{5, 10, 5, 255}         // Deep black-green
	gridColor   = color.RGBA{20, 40, 20, 40}        // Subtle dark green grid
	headColor   = color.RGBA{0, 255, 50, 255}       // Bright lime green
	bodyColor   = color.RGBA{0, 180, 30, 255}       // Forest green
	foodColor   = color.RGBA{255, 50, 50, 255}      // Bright red (highly visible)
	bonusColor  = color.RGBA{255, 255, 100, 255}    // Bright yellow
	shadowColor = color.RGBA{0, 0, 0, 120}

	// Meteor colors - red/orange theme
	meteorColors = []color.RGBA{
		{255, 100, 50, 255},  // Red-orange
		{255, 150, 100, 255}, // Orange
		{255, 200, 150, 255}, // Light orange
		{200, 50, 50, 255},   // Dark red
	}

	// Star colors - green theme with some variety
	starColors = []color.RGBA{
		{200, 255, 200, 255}, // Light green
		{150, 255, 150, 255}, // Medium green
		{100, 200, 100, 255}, // Forest green
		{255, 255, 255, 255}, // White for variety
	}
)

// ==================== INITIALIZATION ====================

func NewGame() *Game {
	g := &Game{
		rng:        rand.New(rand.NewSource(time.Now().UnixNano())),
		menuOption: 0,
		state:      StateTitleScreen,
	}
	
	g.loadGameData()
	g.initializeAudio()
	g.initializeRenderer()
	g.resetGameplay()
	
	return g
}

func (g *Game) initializeAudio() {
	g.audioCtx = audio.NewContext(sampleRate)
	g.eatPlayer = newBeepPlayer(g.audioCtx, 880, 0.1)
	g.comboPlayer = newBeepPlayer(g.audioCtx, 1320, 0.12)
	g.powerUpPlayer = newBeepPlayer(g.audioCtx, 1100, 0.2)
	g.gameOverPlayer = newBeepPlayer(g.audioCtx, 220, 0.5)
	g.bgLoop, g.bgPlayer = newBackgroundLoop(g.audioCtx)
}

func (g *Game) initializeRenderer() {
	g.renderer = &Renderer{game: g}
	g.renderer.initializeBackground()
}

func (g *Game) calculatePlayfieldDimensions() {
	// Calculate optimal grid size based on screen dimensions
	maxCellsW := g.screenWidth / 15  // Minimum cell size of 15 pixels
	maxCellsH := g.screenHeight / 15
	
	// Use aspect ratio to maintain good gameplay
	aspectRatio := float64(g.screenWidth) / float64(g.screenHeight)
	
	if aspectRatio > 1.5 { // Wide screen
		g.gridW = int(math.Min(float64(maxCellsW), 50))
		g.gridH = int(float64(g.gridW) / aspectRatio)
	} else { // Standard or tall screen
		g.gridH = int(math.Min(float64(maxCellsH), 40))
		g.gridW = int(float64(g.gridH) * aspectRatio)
	}
	
	// Ensure minimum playfield size
	if g.gridW < 20 { g.gridW = 20 }
	if g.gridH < 15 { g.gridH = 15 }
	
	// Calculate cell size that fits the screen perfectly
	cellSizeW := g.screenWidth / g.gridW
	cellSizeH := g.screenHeight / g.gridH
	g.cellSize = int(math.Min(float64(cellSizeW), float64(cellSizeH)))
	
	g.scaleFactor = float64(g.cellSize) / float64(baseCellSize)
}

// ==================== AUDIO SYSTEM ====================

func newBeepPlayer(ctx *audio.Context, freq float64, durSec float64) *audio.Player {
	n := int(float64(sampleRate) * durSec)
	buf := make([]byte, n*4)
	for i := 0; i < n; i++ {
		t := float64(i) / sampleRate
		envelope := math.Pow(math.E, -3*t)
		harmonics := math.Sin(2*math.Pi*freq*t) + 
					0.3*math.Sin(2*math.Pi*freq*2*t) +
					0.1*math.Sin(2*math.Pi*freq*3*t)
		v := int16(harmonics * 4000 * envelope)
		for ch := 0; ch < 2; ch++ {
			idx := i*4 + ch*2
			buf[idx] = byte(v)
			buf[idx+1] = byte(v >> 8)
		}
	}
	player := ctx.NewPlayerFromBytes(buf)
	return player
}

func newBackgroundLoop(ctx *audio.Context) (*audio.InfiniteLoop, *audio.Player) {
	// Ambient space-like background music
	notes := []float64{130.81, 146.83, 164.81, 174.61, 196.00, 220.00, 246.94, 261.63}
	durSec := 2.0
	totalSamples := int(float64(sampleRate) * durSec * float64(len(notes)))
	buf := make([]byte, totalSamples*4)
	idx := 0
	
	for _, freq := range notes {
		samplesPerNote := int(float64(sampleRate) * durSec)
		for j := 0; j < samplesPerNote; j++ {
			t := float64(j) / sampleRate
			
			// Create ambient pad sound
			fundamental := math.Sin(2*math.Pi*freq*t)
			fifth := math.Sin(2*math.Pi*freq*1.5*t) * 0.5
			octave := math.Sin(2*math.Pi*freq*2*t) * 0.3
			
			// Slow envelope for ambient feel
			envelope := 0.5 * (1 + math.Sin(2*math.Pi*t/durSec - math.Pi/2))
			if envelope > 1 { envelope = 1 }
			
			// Add some ethereal modulation
			modulation := 1 + 0.1*math.Sin(2*math.Pi*t*0.5)
			
			v := int16((fundamental + fifth + octave) * 800 * envelope * modulation)
			
			for ch := 0; ch < 2; ch++ {
				if idx < len(buf) {
					buf[idx] = byte(v)
					buf[idx+1] = byte(v >> 8)
					idx += 2
				}
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

// ==================== BACKGROUND RENDERER ====================

func (r *Renderer) initializeBackground() {
	// Initialize animated background grid
	r.backgroundGrid = make([][]BackgroundCell, baseGridW*2)
	for x := range r.backgroundGrid {
		r.backgroundGrid[x] = make([]BackgroundCell, baseGridH*2)
		for y := range r.backgroundGrid[x] {
			r.backgroundGrid[x][y] = BackgroundCell{
				intensity:  r.game.rng.Float64() * 0.3, // Reduced for better contrast
				phase:      r.game.rng.Float64() * 2 * math.Pi,
				colorShift: r.game.rng.Float64() * 2 * math.Pi,
			}
		}
	}
	
	// Initialize star field
	starCount := 100 // Reduced for cleaner look
	r.starField = make([]Star, starCount)
	for i := range r.starField {
		r.starField[i] = Star{
			pos: Vector2{
				X: r.game.rng.Float64() * 1920,
				Y: r.game.rng.Float64() * 1080,
			},
			brightness: 0.4 + r.game.rng.Float64()*0.6,
			twinkle:    r.game.rng.Float64() * 2 * math.Pi,
			speed:      0.1 + r.game.rng.Float64()*0.2,
			size:       1 + r.game.rng.Float64()*1.5,
		}
	}
	
	// Initialize meteors
	meteorCount := 12
	r.meteors = make([]Meteor, 0, meteorCount)
}

func (r *Renderer) drawSpaceBackground(screen *ebiten.Image) {
	r.time += 0.016 // Assuming 60 FPS
	
	// Fill with deep space color
	screen.Fill(bgColor)
	
	// Draw stars with twinkling effect
	r.drawStarField(screen)
	
	// Draw meteors
	r.drawMeteors(screen)
	
	// Draw animated grid overlay (more subtle)
	r.drawAnimatedGrid(screen)
}

func (r *Renderer) drawStarField(screen *ebiten.Image) {
	for i := range r.starField {
		star := &r.starField[i]
		
		// Update twinkle effect
		star.twinkle += star.speed * 0.1
		twinkleFactor := 0.7 + 0.3*math.Sin(star.twinkle)
		
		// Calculate final brightness and size
		finalBrightness := star.brightness * twinkleFactor
		finalSize := star.size * (0.8 + 0.4*twinkleFactor)
		
		// Choose star color - mostly green theme
		starColor := starColors[i%len(starColors)]
		alpha := uint8(float64(starColor.A) * finalBrightness)
		finalColor := color.RGBA{starColor.R, starColor.G, starColor.B, alpha}
		
		// Draw star
		ebitenutil.DrawRect(screen,
			star.pos.X - finalSize/2,
			star.pos.Y - finalSize/2,
			finalSize, finalSize, finalColor)
	}
}

func (r *Renderer) drawMeteors(screen *ebiten.Image) {
	// Spawn new meteors occasionally
	if r.game.rng.Float64() < 0.02 && len(r.meteors) < 15 {
		meteor := Meteor{
			pos: Vector2{
				X: -50 + r.game.rng.Float64()*100,
				Y: -50 + r.game.rng.Float64()*100,
			},
			vel: Vector2{
				X: 2 + r.game.rng.Float64()*4,
				Y: 3 + r.game.rng.Float64()*5,
			},
			size:  3 + r.game.rng.Float64()*8,
			color: meteorColors[r.game.rng.Intn(len(meteorColors))],
			trail: make([]Vector2, 0, 20),
			life:  1.0,
			glow:  r.game.rng.Float64(),
		}
		r.meteors = append(r.meteors, meteor)
	}
	
	// Update and draw meteors
	for i := len(r.meteors) - 1; i >= 0; i-- {
		meteor := &r.meteors[i]
		
		// Update position
		meteor.pos.X += meteor.vel.X
		meteor.pos.Y += meteor.vel.Y
		
		// Add to trail
		meteor.trail = append(meteor.trail, meteor.pos)
		if len(meteor.trail) > 15 {
			meteor.trail = meteor.trail[1:]
		}
		
		// Update glow
		meteor.glow += 0.1
		glowFactor := 0.7 + 0.3*math.Sin(meteor.glow)
		
		// Draw trail
		for j, trailPos := range meteor.trail {
			trailAlpha := float64(j) / float64(len(meteor.trail))
			trailSize := meteor.size * 0.3 * trailAlpha
			trailColor := color.RGBA{
				meteor.color.R,
				meteor.color.G,
				meteor.color.B,
				uint8(float64(meteor.color.A) * trailAlpha * 0.5),
			}
			
			if trailSize > 0.5 {
				ebitenutil.DrawRect(screen,
					trailPos.X - trailSize/2,
					trailPos.Y - trailSize/2,
					trailSize, trailSize, trailColor)
			}
		}
		
		// Draw meteor core with glow
		coreSize := meteor.size * glowFactor
		glowSize := coreSize * 2
		
		// Outer glow
		glowColor := color.RGBA{
			meteor.color.R,
			meteor.color.G,
			meteor.color.B,
			uint8(float64(meteor.color.A) * 0.3),
		}
		ebitenutil.DrawRect(screen,
			meteor.pos.X - glowSize/2,
			meteor.pos.Y - glowSize/2,
			glowSize, glowSize, glowColor)
		
		// Core
		ebitenutil.DrawRect(screen,
			meteor.pos.X - coreSize/2,
			meteor.pos.Y - coreSize/2,
			coreSize, coreSize, meteor.color)
		
		// Remove meteors that are off screen
		if meteor.pos.X > float64(r.game.screenWidth)+100 || 
		   meteor.pos.Y > float64(r.game.screenHeight)+100 {
			r.meteors = append(r.meteors[:i], r.meteors[i+1:]...)
		}
	}
}

func (r *Renderer) drawAnimatedGrid(screen *ebiten.Image) {
	if r.game.gridW == 0 || r.game.gridH == 0 {
		return
	}
	
	// Only draw grid during gameplay
	if r.game.state != StatePlaying && r.game.state != StatePaused {
		return
	}
	
	// Calculate grid offset to center the playfield
	offsetX := (r.game.screenWidth - r.game.gridW*r.game.cellSize) / 2
	offsetY := (r.game.screenHeight - r.game.gridH*r.game.cellSize) / 2
	
	// Draw animated background cells within the playfield (very subtle)
	for x := 0; x < r.game.gridW; x++ {
		for y := 0; y < r.game.gridH; y++ {
			// Use modulo to wrap background pattern
			bgX := x % len(r.backgroundGrid)
			bgY := y % len(r.backgroundGrid[0])
			cell := &r.backgroundGrid[bgX][bgY]
			
			// Create wave pattern
			wave := math.Sin(r.time*0.3 + cell.phase + float64(x+y)*0.1)
			
			intensity := cell.intensity + wave*0.05
			if intensity < 0 { intensity = 0 }
			if intensity > 0.3 { intensity = 0.3 } // Very subtle
			
			// Green-tinted background cells
			baseIntensity := int(intensity * 255)
			red := uint8(float64(baseIntensity) * 0.3)
			green := uint8(baseIntensity)
			blue := uint8(float64(baseIntensity) * 0.3)

			
			cellColor := color.RGBA{red, green, blue, 30}
			
			cellX := float64(offsetX + x*r.game.cellSize)
			cellY := float64(offsetY + y*r.game.cellSize)
			cellSize := float64(r.game.cellSize)
			
			ebitenutil.DrawRect(screen, cellX, cellY, cellSize, cellSize, cellColor)
		}
	}
	
	// Draw grid lines with subtle green glow
	gridAlpha := uint8(40 + 10*math.Sin(r.time*0.3))
	gridColor := color.RGBA{30, 60, 30, gridAlpha}
	
	// Vertical lines
	for x := 0; x <= r.game.gridW; x++ {
		lineX := float64(offsetX + x*r.game.cellSize)
		ebitenutil.DrawRect(screen, lineX-0.5, float64(offsetY), 1, float64(r.game.gridH*r.game.cellSize), gridColor)
	}
	
	// Horizontal lines
	for y := 0; y <= r.game.gridH; y++ {
		lineY := float64(offsetY + y*r.game.cellSize)
		ebitenutil.DrawRect(screen, float64(offsetX), lineY-0.5, float64(r.game.gridW*r.game.cellSize), 1, gridColor)
	}
}

// ==================== GAME STATE MANAGEMENT ====================

func (g *Game) resetGameplay() {
	g.calculatePlayfieldDimensions()
	
	midX, midY := g.gridW/2, g.gridH/2
	g.snake = []Point{{midX, midY}, {midX-1, midY}, {midX-2, midY}}
	g.dir = Point{1, 0}
	g.nextDir = g.dir
	g.grow = 0
	g.frame = 0
	g.score = 0
	g.combo = 0
	g.maxCombo = 0
	g.comboTimer = 0
	g.state = StatePlaying
	g.foodPulse = 0
	g.headPulse = 0
	g.speedBoostTime = 0
	g.slowMotionTime = 0
	g.invulnerable = 0
	g.shakeIntensity = 0
	g.speed = g.baseSpeed
	g.particles = g.particles[:0]
	g.trailOpacity = make([]float64, len(g.snake))
	g.powerUp = PowerUp{}
	g.gameStartTime = time.Now()
	g.baseSpeed = 10
	
	g.placeFood()
	g.bgPlayer.Rewind()
	g.bgPlayer.Play()
}

func (g *Game) loadGameData() {
	data, err := os.ReadFile(saveFile)
	if err != nil {
		g.gameData = GameData{}
		return
	}
	json.Unmarshal(data, &g.gameData)
}

func (g *Game) saveGameData() {
	data, _ := json.Marshal(g.gameData)
	os.WriteFile(saveFile, data, 0644)
}

// ==================== GAME LOGIC ====================

func (g *Game) placeFood() {
	for {
		f := Point{g.rng.Intn(g.gridW), g.rng.Intn(g.gridH)}
		occupied := false
		for _, s := range g.snake {
			if s == f {
				occupied = true
				break
			}
		}
		if !occupied && (g.powerUp.pos != f || !g.powerUp.active) {
			g.food = f
			return
		}
	}
}

func (g *Game) placePowerUp() {
	if g.powerUp.active || g.rng.Float64() > 0.15 {
		return
	}
	
	for {
		p := Point{g.rng.Intn(g.gridW), g.rng.Intn(g.gridH)}
		if p == g.food {
			continue
		}
		occupied := false
		for _, s := range g.snake {
			if s == p {
				occupied = true
				break
			}
		}
		if !occupied {
			g.powerUp = PowerUp{
				pos:    p,
				type_:  g.rng.Intn(3),
				timer:  600, // 10 seconds at 60fps
				active: true,
				pulse:  0,
				sparkles: make([]Particle, 0, 10),
			}
			return
		}
	}
}

func (g *Game) addParticles(pos Point, count int, particleColor color.RGBA) {
	// Calculate screen position considering playfield offset
	offsetX := (g.screenWidth - g.gridW*g.cellSize) / 2
	offsetY := (g.screenHeight - g.gridH*g.cellSize) / 2
	
	screenX := float64(offsetX + pos.X*g.cellSize + g.cellSize/2)
	screenY := float64(offsetY + pos.Y*g.cellSize + g.cellSize/2)
	
	for i := 0; i < count; i++ {
		angle := float64(i) * 2 * math.Pi / float64(count) + g.rng.Float64()*0.5
		speed := 2.0 + g.rng.Float64()*4.0
		g.particles = append(g.particles, Particle{
			pos:      Vector2{screenX, screenY},
			vel:      Vector2{math.Cos(angle) * speed, math.Sin(angle) * speed},
			life:     1.0,
			maxLife:  0.8 + g.rng.Float64()*0.4,
			color:    particleColor,
			size:     2.0 + g.rng.Float64()*3.0,
			rotation: g.rng.Float64() * 2 * math.Pi,
			rotVel:   (g.rng.Float64() - 0.5) * 0.3,
		})
	}
}

func (g *Game) updateParticles() {
	for i := len(g.particles) - 1; i >= 0; i-- {
		p := &g.particles[i]
		p.pos.X += p.vel.X
		p.pos.Y += p.vel.Y
		p.vel.X *= 0.98
		p.vel.Y *= 0.98
		p.rotation += p.rotVel
		p.life -= 1.0 / 60.0 / p.maxLife
		p.size *= 0.99
		
		if p.life <= 0 || p.size < 0.5 {
			g.particles = append(g.particles[:i], g.particles[i+1:]...)
		}
	}
}

// ==================== MAIN UPDATE FUNCTION ====================

func (g *Game) Update() error {
	g.handleGlobalInput()
	
	switch g.state {
	case StateTitleScreen:
		return g.updateTitleScreen()
	case StateMenu:
		return g.updateMenu()
	case StatePlaying:
		return g.updateGameplay()
	case StatePaused:
		return g.updatePaused()
	case StateGameOver:
		return g.updateGameOver()
	}
	
	return nil
}

func (g *Game) handleGlobalInput() {
	// Fullscreen toggle
	if inpututil.IsKeyJustPressed(ebiten.KeyF11) {
		g.isFullscreen = !g.isFullscreen
		ebiten.SetFullscreen(g.isFullscreen)
	}

	// Escape key handling
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		switch g.state {
		case StatePlaying:
			g.state = StateMenu
			g.bgPlayer.Pause()
		case StatePaused:
			g.state = StateMenu
		case StateMenu:
			if g.score > 0 { // Game in progress
				g.state = StatePlaying
				g.bgPlayer.Play()
			} else {
				g.state = StateTitleScreen
			}
		}
	}
}

func (g *Game) updateTitleScreen() error {
	g.renderer.time += 0.016
	
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		g.resetGameplay()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyS) {
		g.state = StateMenu
		g.menuOption = 2 // Statistics option
	}
	return nil
}

func (g *Game) updateMenu() error {
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) || inpututil.IsKeyJustPressed(ebiten.KeyW) {
		g.menuOption = (g.menuOption - 1 + 4) % 4
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) || inpututil.IsKeyJustPressed(ebiten.KeyS) {
		g.menuOption = (g.menuOption + 1) % 4
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		switch g.menuOption {
		case 0: // Resume/New Game
			if g.state == StateGameOver || g.score == 0 {
				g.resetGameplay()
			} else {
				g.state = StatePlaying
				g.bgPlayer.Play()
			}
		case 1: // New Game
			g.resetGameplay()
		case 2: // Reset Stats
			g.gameData = GameData{}
			g.saveGameData()
		case 3: // Back to Title
			g.state = StateTitleScreen
			g.bgPlayer.Pause()
		}
	}
	return nil
}

func (g *Game) updatePaused() error {
	if inpututil.IsKeyJustPressed(ebiten.KeyP) {
		g.state = StatePlaying
		g.bgPlayer.Play()
	}
	return nil
}

func (g *Game) updateGameOver() error {
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyR) {
		// Update stats
		g.gameData.TotalGames++
		g.gameData.TotalScore += g.score
		if g.score > g.gameData.HighScore {
			g.gameData.HighScore = g.score
		}
		if g.maxCombo > g.gameData.BestCombo {
			g.gameData.BestCombo = g.maxCombo
		}
		g.gameData.PlayTime += int64(time.Since(g.gameStartTime).Seconds())
		g.saveGameData()
		g.resetGameplay()
	}
	return nil
}

func (g *Game) updateGameplay() error {
	// Pause toggle
	if inpututil.IsKeyJustPressed(ebiten.KeyP) {
		g.state = StatePaused
		g.bgPlayer.Pause()
		return nil
	}

	// Speed controls
	if inpututil.IsKeyJustPressed(ebiten.KeyEqual) || inpututil.IsKeyJustPressed(ebiten.KeyNumpadAdd) {
		if g.baseSpeed > minSpeed {
			g.baseSpeed--
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyMinus) {
		if g.baseSpeed < maxSpeed {
			g.baseSpeed++
		}
	}

	// Movement input
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

	// Update game state
	g.frame++
	g.foodPulse += 0.08
	g.headPulse += 0.1
	g.renderer.time += 0.016
	
	// Update timers
	if g.speedBoostTime > 0 {
		g.speedBoostTime--
		g.speed = g.baseSpeed / 2
	} else if g.slowMotionTime > 0 {
		g.slowMotionTime--
		g.speed = g.baseSpeed * 2
	} else {
		g.speed = g.baseSpeed
	}
	
	if g.invulnerable > 0 {
		g.invulnerable--
	}
	
	if g.shakeIntensity > 0 {
		g.shakeIntensity *= 0.9
	}

	// Update power-up
	if g.powerUp.active {
		g.powerUp.timer--
		g.powerUp.pulse += 0.12
		
		// Add sparkle effects to power-ups
		if g.frame % 10 == 0 {
			sparkleColor := bonusColor
			switch g.powerUp.type_ {
			case 1: sparkleColor = color.RGBA{100, 255, 100, 255}
			case 2: sparkleColor = color.RGBA{100, 100, 255, 255}
			}
			g.addParticles(g.powerUp.pos, 1, sparkleColor)
		}
		
		if g.powerUp.timer <= 0 {
			g.powerUp.active = false
		}
	} else if g.frame % 300 == 0 { // Try to spawn power-up every 5 seconds
		g.placePowerUp()
	}

	g.updateParticles()

	// Game movement logic
	if g.frame%g.speed != 0 {
		return nil
	}

	g.dir = g.nextDir
	head := g.snake[0]
	newHead := Point{(head.X + g.dir.X + g.gridW) % g.gridW, (head.Y + g.dir.Y + g.gridH) % g.gridH}

	// Check collision with snake body
	if g.invulnerable == 0 {
		for _, s := range g.snake {
			if s == newHead {
				g.state = StateGameOver
				g.gameOverPlayer.Rewind()
				g.gameOverPlayer.Play()
				g.shakeIntensity = 15.0
				g.addParticles(newHead, 15, color.RGBA{255, 100, 100, 255})
				return nil
			}
		}
	}

	// Move snake
	g.snake = append([]Point{newHead}, g.snake...)
	
	// Update trail opacity
	if len(g.trailOpacity) != len(g.snake) {
		g.trailOpacity = make([]float64, len(g.snake))
	}
	for i := range g.trailOpacity {
		g.trailOpacity[i] = 1.0 - float64(i)/float64(len(g.snake))
	}

	// Check food collision
	if newHead == g.food {
		g.grow += 2
		g.combo++
		if g.combo > g.maxCombo {
			g.maxCombo = g.combo
		}
		g.comboTimer = 120 // 2 seconds
		basePoints := 1
		comboBonus := g.combo / 3
		g.score += basePoints + comboBonus
		
		// Play appropriate sound
		if g.combo > 3 {
			g.comboPlayer.Rewind()
			g.comboPlayer.Play()
		} else {
			g.eatPlayer.Rewind()
			g.eatPlayer.Play()
		}
		
		// Add particles - red for food
		particleCount := 8 + g.combo/2
		g.addParticles(g.food, particleCount, foodColor)
		
		g.placeFood()
	} else {
		g.comboTimer--
		if g.comboTimer <= 0 {
			g.combo = 0
		}
	}

	// Check power-up collision
	if g.powerUp.active && newHead == g.powerUp.pos {
		g.powerUpPlayer.Rewind()
		g.powerUpPlayer.Play()
		g.addParticles(g.powerUp.pos, 12, bonusColor)
		
		switch g.powerUp.type_ {
		case 0: // Bonus points
			g.score += 5 + g.combo
		case 1: // Speed boost
			g.speedBoostTime = 300 // 5 seconds
		case 2: // Invulnerability
			g.invulnerable = 180 // 3 seconds
		}
		g.powerUp.active = false
	}

	// Grow or shrink snake
	if g.grow > 0 {
		g.grow--
	} else if len(g.snake) > 1 {
		g.snake = g.snake[:len(g.snake)-1]
	}

	return nil
}

// ==================== RENDERING SYSTEM ====================

func (g *Game) drawEnhancedCell(screen *ebiten.Image, x, y int, c color.RGBA, scale float64, opacity float64) {
	if opacity <= 0 {
		return
	}
	
	// Calculate screen position with centering offset
	offsetX := (g.screenWidth - g.gridW*g.cellSize) / 2
	offsetY := (g.screenHeight - g.gridH*g.cellSize) / 2
	
	size := float64(g.cellSize) * scale
	cellOffset := float64(g.cellSize) * (1-scale) / 2
	posX := float64(offsetX + x*g.cellSize) + cellOffset
	posY := float64(offsetY + y*g.cellSize) + cellOffset
	
	// Apply screen shake
	if g.shakeIntensity > 0 {
		posX += (g.rng.Float64() - 0.5) * g.shakeIntensity
		posY += (g.rng.Float64() - 0.5) * g.shakeIntensity
	}
	
	// Draw shadow first
	shadowOffset := 2.0 * g.scaleFactor
	shadowColor := color.RGBA{0, 0, 0, uint8(float64(shadowColor.A) * opacity * 0.3)}
	ebitenutil.DrawRect(screen, posX+shadowOffset, posY+shadowOffset, size, size, shadowColor)
	
	// Apply opacity to main color
	finalColor := color.RGBA{c.R, c.G, c.B, uint8(float64(c.A) * opacity)}
	
	// Draw main cell
	ebitenutil.DrawRect(screen, posX, posY, size, size, finalColor)
	
	// Highlight effect for certain cells
	if scale > 0.95 {
		highlightColor := color.RGBA{
			uint8(math.Min(255, float64(c.R)+80)),
			uint8(math.Min(255, float64(c.G)+80)),
			uint8(math.Min(255, float64(c.B)+80)),
			uint8(float64(c.A) * opacity * 0.6),
		}
		highlightSize := size * 0.4
		highlightOffset := size * 0.1
		ebitenutil.DrawRect(screen, posX+highlightOffset, posY+highlightOffset, highlightSize, highlightSize, highlightColor)
	}
}

func (g *Game) drawParticles(screen *ebiten.Image) {
	for _, p := range g.particles {
		if p.life > 0 {
			alpha := float32(p.color.A) * float32(p.life)
			particleColor := color.RGBA{p.color.R, p.color.G, p.color.B, uint8(alpha)}

			x := p.pos.X
			y := p.pos.Y
			size := p.size

			// Apply screen shake to particles too
			if g.shakeIntensity > 0 {
				x += (g.rng.Float64() - 0.5) * g.shakeIntensity * 0.5
				y += (g.rng.Float64() - 0.5) * g.shakeIntensity * 0.5
			}

			// Simple rectangular particle for now
			ebitenutil.DrawRect(screen,
				x - size/2,
				y - size/2,
				size, size, particleColor)
		}
	}
}

func (g *Game) Draw(screen *ebiten.Image) {
	// Always draw the space background
	g.renderer.drawSpaceBackground(screen)
	
	switch g.state {
	case StateTitleScreen:
		g.drawTitleScreen(screen)
	case StateMenu:
		g.drawMenuScreen(screen)
	case StatePlaying, StatePaused, StateGameOver:
		g.drawGameplay(screen)
		if g.state == StatePaused {
			g.drawPauseOverlay(screen)
		} else if g.state == StateGameOver {
			g.drawGameOverOverlay(screen)
		}
	}
}

func (g *Game) drawGameplay(screen *ebiten.Image) {
	// Draw power-up
	if g.powerUp.active {
		pulse := 0.8 + 0.2*math.Sin(g.powerUp.pulse)
		var powerColor color.RGBA
		switch g.powerUp.type_ {
		case 0: powerColor = bonusColor
		case 1: powerColor = color.RGBA{120, 255, 120, 255}
		case 2: powerColor = color.RGBA{120, 120, 255, 255}
		}
		g.drawEnhancedCell(screen, g.powerUp.pos.X, g.powerUp.pos.Y, powerColor, pulse, 1.0)
	}

	// Draw food with enhanced visibility - bright red with white border
	pulse := 1.0 + 0.2*math.Sin(g.foodPulse*2) // Stronger pulse for visibility
	
	// Draw white border for maximum visibility
	borderColor := color.RGBA{255, 255, 255, 200}
	g.drawEnhancedCell(screen, g.food.X, g.food.Y, borderColor, pulse*1.2, 1.0)
	
	// Draw bright red core
	currentFoodColor := foodColor
	if g.combo > 0 {
		// Alternate between bright red and bright yellow for combo
		if int(g.foodPulse*4)%2 == 0 {
			currentFoodColor = color.RGBA{255, 255, 50, 255} // Bright yellow
		} else {
			currentFoodColor = color.RGBA{255, 50, 50, 255}  // Bright red
		}
	}
	g.drawEnhancedCell(screen, g.food.X, g.food.Y, currentFoodColor, pulse, 1.0)

	// Draw snake with green theme
	for i, s := range g.snake {
		opacity := 1.0
		if i < len(g.trailOpacity) {
			opacity = g.trailOpacity[i]
		}
		
		if i == 0 {
			// Enhanced head with pulsing effect
			headScale := 1.0 + 0.1*math.Sin(g.headPulse)
			currentHeadColor := headColor
			
			// Special effects based on power-ups
			if g.invulnerable > 0 {
				// Flashing invulnerability - green/white
				if (g.frame/5)%2 == 0 {
					currentHeadColor = color.RGBA{200, 255, 200, 255}
				}
			} else if g.speedBoostTime > 0 {
				currentHeadColor = color.RGBA{150, 255, 100, 255} // Brighter green
			} else if g.slowMotionTime > 0 {
				currentHeadColor = color.RGBA{100, 150, 100, 255} // Darker green
			}
			
			g.drawEnhancedCell(screen, s.X, s.Y, currentHeadColor, headScale, opacity)
		} else {
			// Body with gradient effect
			bodyScale := 0.9 - float64(i)*0.01
			if bodyScale < 0.5 { bodyScale = 0.5 }
			
			// Gradient body color - green theme
			factor := float64(i) / float64(len(g.snake))
			currentBodyColor := color.RGBA{
				uint8(float64(bodyColor.R) * (1 - factor*0.4)),
				uint8(float64(bodyColor.G) * (1 - factor*0.3)),
				uint8(float64(bodyColor.B) * (1 - factor*0.4)),
				bodyColor.A,
			}
			
			g.drawEnhancedCell(screen, s.X, s.Y, currentBodyColor, bodyScale, opacity)
		}
	}

	// Draw particles
	g.drawParticles(screen)

	// Draw HUD
	g.drawHUD(screen)
}

func (g *Game) drawTitleScreen(screen *ebiten.Image) {
	centerX := float64(g.screenWidth) / 2
	centerY := float64(g.screenHeight) / 2

	lines := []string{
		"🌌 COSMIC SNAKE 🐍",
		"",
		"🔥 Meteor Storm Edition",
		"• Dynamic Fullscreen Arena",
		"• Falling Meteor Background",
		"• Green/Black/Red Theme",
		"• Enhanced Food Visibility",
		"• Combo System & Power-ups",
		"• Spectacular Visual Effects",
		"",
		"🎯 Controls:",
		"Arrow Keys/WASD: Move",
		"P: Pause | F11: Fullscreen | Esc: Menu",
		"+/-: Speed Control",
		"",
		"🏆 Statistics:",
		fmt.Sprintf("High Score: %d | Games: %d", g.gameData.HighScore, g.gameData.TotalGames),
		fmt.Sprintf("Best Combo: %d", g.gameData.BestCombo),
		"",
		"🚀 Press ENTER/SPACE to Launch!",
		"Press S for Statistics",
	}

	lineHeight := 22.0
	totalHeight := float64(len(lines)) * lineHeight
	startY := centerY - totalHeight/2

	face := basicfont.Face7x13

	for i, line := range lines {
		if line == "" {
			continue
		}

		approxWidth := float64(len(line)) * 8
		x := centerX - approxWidth/2
		y := startY + float64(i)*lineHeight

		// Decide line color based on green/black/red theme
		var lineColor color.Color = color.RGBA{200, 255, 200, 255} // Light green default
		switch {
		case i == 0: // Title
			lineColor = color.RGBA{0, 255, 100, 255} // Bright green title
		case i == 2: // "Meteor Storm Edition"
			lineColor = color.RGBA{255, 100, 100, 255} // Red accent
		case i >= 3 && i <= 8: // feature list
			lineColor = color.RGBA{150, 255, 150, 255} // Light green
		case i == 10: // "Controls"
			lineColor = color.RGBA{255, 200, 100, 255} // Yellow-orange
		case i >= 11 && i <= 13: // control list
			lineColor = color.RGBA{200, 200, 255, 255} // Light blue
		case i == 15: // "Statistics"
			lineColor = color.RGBA{255, 150, 150, 255} // Light red
		case i >= 16 && i <= 17: // stats values
			lineColor = color.RGBA{255, 255, 200, 255} // Light yellow
		case i >= 19 && i <= 20: // Launch instructions
			lineColor = color.RGBA{100, 255, 100, 255} // Bright green
		}

		// Draw line with chosen color
		text.Draw(screen, line, face, int(x), int(y), lineColor)
	}
}

func (g *Game) drawMenuScreen(screen *ebiten.Image) {
	// Semi-transparent overlay
	overlay := ebiten.NewImage(g.screenWidth, g.screenHeight)
	overlay.Fill(color.RGBA{0, 0, 0, 180})
	screen.DrawImage(overlay, nil)

	centerX := float64(g.screenWidth) / 2
	centerY := float64(g.screenHeight) / 2

	menuItems := []string{
		"Resume Game",
		"New Game",
		"Reset Statistics",
		"Back to Title",
	}

	if g.state == StateGameOver || g.score == 0 {
		menuItems[0] = "Start New Game"
	}

	lineHeight := 40.0
	totalHeight := float64(len(menuItems)) * lineHeight
	startY := centerY - totalHeight/2

	// Menu title
	title := "=== COSMIC MENU ==="
	if g.state == StateGameOver {
		title = "=== MISSION COMPLETE ==="
	}
	titleWidth := float64(len(title)) * 10
	titleX := centerX - titleWidth/2
	titleY := startY - 80

	face := basicfont.Face7x13

	// Title in green
	text.Draw(screen, title, face, int(titleX), int(titleY), color.RGBA{100, 255, 100, 255})

	// Menu items
	for i, item := range menuItems {
		approxWidth := float64(len(item)) * 9
		x := centerX - approxWidth/2
		y := startY + float64(i)*lineHeight

		if i == g.menuOption {
			// Selected option in bright green
			prefix := "► "
			suffix := " ◄"
			fullText := prefix + item + suffix
			fullWidth := float64(len(fullText)) * 9
			fullX := centerX - fullWidth/2

			text.Draw(screen, fullText, face, int(fullX), int(y), color.RGBA{0, 255, 100, 255})
		} else {
			text.Draw(screen, item, face, int(x), int(y), color.RGBA{150, 255, 150, 255})
		}
	}

	// Show current game stats if in game
	if g.state != StateGameOver && g.score > 0 {
		statsY := startY + float64(len(menuItems))*lineHeight + 60
		stats := []string{
			fmt.Sprintf("Current Score: %d", g.score),
			fmt.Sprintf("Current Combo: %d (Max: %d)", g.combo, g.maxCombo),
			fmt.Sprintf("Snake Length: %d", len(g.snake)),
			fmt.Sprintf("Playfield: %dx%d", g.gridW, g.gridH),
		}

		for i, stat := range stats {
			statWidth := float64(len(stat)) * 8
			statX := centerX - statWidth/2
			statY := statsY + float64(i)*25
			text.Draw(screen, stat, face, int(statX), int(statY), color.RGBA{200, 255, 200, 255})
		}
	}
}

func (g *Game) drawPauseOverlay(screen *ebiten.Image) {
	// Semi-transparent overlay
	overlay := ebiten.NewImage(g.screenWidth, g.screenHeight)
	overlay.Fill(color.RGBA{0, 0, 0, 120})
	screen.DrawImage(overlay, nil)

	centerX := float64(g.screenWidth) / 2
	centerY := float64(g.screenHeight) / 2

	pauseText := "⏸️ PAUSED"
	textWidth := float64(len(pauseText)) * 15

	face := basicfont.Face7x13

	// Main pause text in green
	text.Draw(screen, pauseText, face, int(centerX-textWidth/2), int(centerY), color.RGBA{100, 255, 100, 255})

	// Instructions
	instruction := "Press P to Resume or ESC for Menu"
	instrWidth := float64(len(instruction)) * 8
	text.Draw(screen, instruction, face, int(centerX-instrWidth/2), int(centerY+40), color.RGBA{200, 255, 200, 255})
}

func (g *Game) drawGameOverOverlay(screen *ebiten.Image) {
	// Semi-transparent red overlay
	overlay := ebiten.NewImage(g.screenWidth, g.screenHeight)
	overlay.Fill(color.RGBA{50, 0, 0, 150})
	screen.DrawImage(overlay, nil)

	centerX := float64(g.screenWidth) / 2
	centerY := float64(g.screenHeight) / 2

	face := basicfont.Face7x13

	// Game Over text in red
	gameOverText := "💀 MISSION FAILED 💀"
	textWidth := float64(len(gameOverText)) * 12
	text.Draw(screen, gameOverText, face, int(centerX-textWidth/2), int(centerY-50), color.RGBA{255, 100, 100, 255})

	// Final score in white
	finalScore := fmt.Sprintf("Final Score: %d", g.score)
	scoreWidth := float64(len(finalScore)) * 10
	text.Draw(screen, finalScore, face, int(centerX-scoreWidth/2), int(centerY), color.White)

	// High score notification
	if g.score > g.gameData.HighScore {
		newRecord := "🏆 NEW HIGH SCORE! 🏆"
		recordWidth := float64(len(newRecord)) * 10
		text.Draw(screen, newRecord, face, int(centerX-recordWidth/2), int(centerY+30), color.RGBA{255, 255, 100, 255})
	}

	// Instructions
	instruction := "Press ENTER/R to Restart or ESC for Menu"
	instrWidth := float64(len(instruction)) * 8
	text.Draw(screen, instruction, face, int(centerX-instrWidth/2), int(centerY+80), color.RGBA{200, 255, 200, 255})
}

func (g *Game) drawHUD(screen *ebiten.Image) {
	padding := 15.0
	lineHeight := 18.0
	
	// Main HUD with green theme
	lines := []string{
		fmt.Sprintf("Score: %d | High: %d | Speed: %d", g.score, g.gameData.HighScore, maxSpeed-g.baseSpeed+minSpeed),
		fmt.Sprintf("Length: %d | Combo: %dx (Best: %dx)", len(g.snake), g.combo, g.maxCombo),
		fmt.Sprintf("Arena: %dx%d", g.gridW, g.gridH),
	}
	
	// Status effects with icons
	var effects []string
	if g.speedBoostTime > 0 {
		effects = append(effects, fmt.Sprintf("🚀 BOOST: %ds", g.speedBoostTime/60+1))
	}
	if g.slowMotionTime > 0 {
		effects = append(effects, fmt.Sprintf("🐌 SLOW: %ds", g.slowMotionTime/60+1))
	}
	if g.invulnerable > 0 {
		effects = append(effects, fmt.Sprintf("🛡️ SHIELD: %ds", g.invulnerable/60+1))
	}
	
	// Power-up indicator
	if g.powerUp.active {
		powerUpNames := []string{"💰 BONUS", "🚀 SPEED", "🛡️ SHIELD"}
		effects = append(effects, fmt.Sprintf("%s: %ds", powerUpNames[g.powerUp.type_], g.powerUp.timer/60+1))
	}
	
	lines = append(lines, effects...)
	
	// Controls hint for new players
	if g.frame < 360 { // Show for first 6 seconds
		lines = append(lines, "F11: Fullscreen | ESC: Menu | P: Pause | +/-: Speed")
	}
	
	// Draw HUD with dark background
	hudHeight := float64(len(lines)) * lineHeight + padding*2
	hudBg := color.RGBA{0, 20, 0, 150} // Dark green background
	ebitenutil.DrawRect(screen, 0, 0, 400, hudHeight, hudBg)
	
	for i, line := range lines {
		y := padding + float64(i)*lineHeight
		// Draw text in bright green
		face := basicfont.Face7x13
		text.Draw(screen, line, face, int(padding), int(y), color.RGBA{100, 255, 100, 255})
	}
	
	// Progress bars for effects
	barY := padding + float64(len(lines))*lineHeight + 10
	barWidth := 250.0
	barHeight := 6.0
	
	if g.speedBoostTime > 0 {
		progress := float64(g.speedBoostTime) / 300.0
		// Background
		ebitenutil.DrawRect(screen, padding, barY, barWidth, barHeight, color.RGBA{20, 20, 20, 180})
		// Progress in bright green
		ebitenutil.DrawRect(screen, padding, barY, barWidth*progress, barHeight, color.RGBA{100, 255, 100, 255})
		barY += barHeight + 8
	}
	
	if g.invulnerable > 0 {
		progress := float64(g.invulnerable) / 180.0
		// Background
		ebitenutil.DrawRect(screen, padding, barY, barWidth, barHeight, color.RGBA{20, 20, 20, 180})
		// Progress in blue
		ebitenutil.DrawRect(screen, padding, barY, barWidth*progress, barHeight, color.RGBA{100, 150, 255, 255})
	}
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	g.screenWidth = outsideWidth
	g.screenHeight = outsideHeight
	
	// Recalculate playfield dimensions when window size changes
	if g.state == StatePlaying || g.state == StatePaused {
		g.calculatePlayfieldDimensions()
	}
	
	return outsideWidth, outsideHeight
}

// ==================== MAIN FUNCTION ====================

func main() {
	ebiten.SetWindowSize(1280, 720)
	ebiten.SetWindowTitle("Cosmic Snake - Meteor Storm Edition")
	ebiten.SetWindowResizable(true)
	ebiten.SetWindowSizeLimits(800, 600, -1, -1)
	
	// Start in fullscreen for the best experience
	ebiten.SetFullscreen(true)
	
	game := NewGame()
	game.isFullscreen = true
	
	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}