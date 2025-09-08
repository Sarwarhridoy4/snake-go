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
	minSpeed   = 4
	maxSpeed   = 20
	sampleRate = 44100
	saveFile   = "snake_enhanced.json"
)

var (
	bgColor     = color.RGBA{15, 15, 20, 255}
	gridColor   = color.RGBA{30, 30, 40, 255}
	headColor   = color.RGBA{100, 255, 150, 255}
	bodyColor   = color.RGBA{70, 200, 120, 255}
	foodColor   = color.RGBA{255, 80, 80, 255}
	bonusColor  = color.RGBA{255, 215, 0, 255}
	shadowColor = color.RGBA{0, 0, 0, 100}
)

type Point struct{ X, Y int }
type Vector2 struct{ X, Y float64 }

type Particle struct {
	pos      Vector2
	vel      Vector2
	life     float64
	maxLife  float64
	color    color.RGBA
	size     float64
}

type PowerUp struct {
	pos      Point
	type_    int // 0: bonus points, 1: speed boost, 2: slow motion
	timer    int
	active   bool
	pulse    float64
}

type GameData struct {
	HighScore    int   `json:"high_score"`
	TotalGames   int   `json:"total_games"`
	TotalScore   int   `json:"total_score"`
	BestCombo    int   `json:"best_combo"`
	PlayTime     int64 `json:"play_time_seconds"`
}

type Game struct {
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
	gameOver       bool
	paused         bool
	inTitle        bool
	inMenu         bool
	menuOption     int
	combo          int
	maxCombo       int
	comboTimer     int
	foodPulse      float64
	scaleFactor    float64
	isFullscreen   bool
	gameStartTime  time.Time
	speedBoostTime int
	slowMotionTime int
	invulnerable   int
	shakeIntensity float64

	// Enhanced visual effects
	trailOpacity   []float64
	headPulse      float64
	backgroundWave float64
	
	// Audio
	audioCtx       *audio.Context
	eatPlayer      *audio.Player
	comboPlayer    *audio.Player
	powerUpPlayer  *audio.Player
	gameOverPlayer *audio.Player
	bgLoop         *audio.InfiniteLoop
	bgPlayer       *audio.Player
}

func NewGame() *Game {
	g := &Game{
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
		menuOption: 0,
	}
	g.loadGameData()

	g.audioCtx = audio.NewContext(sampleRate)
	g.eatPlayer = newBeepPlayer(g.audioCtx, 880, 0.1)
	g.comboPlayer = newBeepPlayer(g.audioCtx, 1320, 0.12)
	g.powerUpPlayer = newBeepPlayer(g.audioCtx, 1100, 0.2)
	g.gameOverPlayer = newBeepPlayer(g.audioCtx, 220, 0.5)
	g.bgLoop, g.bgPlayer = newBackgroundLoop(g.audioCtx)

	g.inTitle = true
	g.inMenu = false
	g.baseSpeed = 10
	g.speed = g.baseSpeed
	g.scaleFactor = 1.0
	g.isFullscreen = false
	g.particles = make([]Particle, 0, 100)
	return g
}

func newBeepPlayer(ctx *audio.Context, freq float64, durSec float64) *audio.Player {
	n := int(float64(sampleRate) * durSec)
	buf := make([]byte, n*4)
	for i := 0; i < n; i++ {
		t := float64(i) / sampleRate
		envelope := math.Pow(math.E, -3*t)
		v := int16(math.Sin(2*math.Pi*freq*t) * 6000 * envelope)
		for ch := 0; ch < 2; ch++ {
			idx := i*4 + ch*2
			buf[idx] = byte(v)
			buf[idx+1] = byte(v >> 8)
		}
	}
	return ctx.NewPlayerFromBytes(buf)
}

func newBackgroundLoop(ctx *audio.Context) (*audio.InfiniteLoop, *audio.Player) {
	// More complex melody
	notes := []float64{261.63, 329.63, 392.00, 523.25, 493.88, 440.00, 392.00, 329.63}
	durSec := 0.4
	totalSamples := int(float64(sampleRate) * durSec * float64(len(notes)))
	buf := make([]byte, totalSamples*4)
	idx := 0
	
	for i, freq := range notes {
		for j := 0; j < int(float64(sampleRate)*durSec); j++ {
			t := float64(j) / sampleRate
			// Add harmony
			harmony := freq * 1.25
			envelope := math.Pow(math.E, -1.5*t)
			v1 := math.Sin(2*math.Pi*freq*t) * 1500 * envelope
			v2 := math.Sin(2*math.Pi*harmony*t) * 800 * envelope
			// Add slight variation based on note position
			variation := math.Sin(2*math.Pi*freq*t*float64(i+1)*0.1) * 300 * envelope
			v := int16(v1 + v2 + variation)
			
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
	g.maxCombo = 0
	g.comboTimer = 0
	g.gameOver = false
	g.paused = false
	g.inTitle = false
	g.inMenu = false
	g.foodPulse = 0
	g.headPulse = 0
	g.backgroundWave = 0
	g.speedBoostTime = 0
	g.slowMotionTime = 0
	g.invulnerable = 0
	g.shakeIntensity = 0
	g.speed = g.baseSpeed
	g.particles = g.particles[:0]
	g.trailOpacity = make([]float64, len(g.snake))
	g.powerUp = PowerUp{}
	g.gameStartTime = time.Now()
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

func (g *Game) placePowerUp() {
	if g.powerUp.active || g.rng.Float64() > 0.15 {
		return
	}
	
	for {
		p := Point{g.rng.Intn(gridW), g.rng.Intn(gridH)}
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
			}
			return
		}
	}
}

func (g *Game) addParticles(pos Point, count int, particleColor color.RGBA) {
	for i := 0; i < count; i++ {
		angle := float64(i) * 2 * math.Pi / float64(count) + g.rng.Float64()*0.5
		speed := 2.0 + g.rng.Float64()*3.0
		g.particles = append(g.particles, Particle{
			pos:     Vector2{float64(pos.X*cellSize + cellSize/2), float64(pos.Y*cellSize + cellSize/2)},
			vel:     Vector2{math.Cos(angle) * speed, math.Sin(angle) * speed},
			life:    1.0,
			maxLife: 0.8 + g.rng.Float64()*0.4,
			color:   particleColor,
			size:    3.0 + g.rng.Float64()*2.0,
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
		p.life -= 1.0 / 60.0 / p.maxLife
		p.size *= 0.99
		
		if p.life <= 0 || p.size < 0.5 {
			g.particles = append(g.particles[:i], g.particles[i+1:]...)
		}
	}
}

func (g *Game) Update() error {
	// Global controls
	if inpututil.IsKeyJustPressed(ebiten.KeyF) {
		g.isFullscreen = !g.isFullscreen
		if g.isFullscreen {
			ebiten.MaximizeWindow()
		} else {
			ebiten.RestoreWindow()
			ebiten.SetWindowSize(1280, 720)
		}
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		if g.isFullscreen {
			g.isFullscreen = false
			ebiten.RestoreWindow()
			ebiten.SetWindowSize(1280, 720)
		} else if !g.inTitle && !g.gameOver {
			g.inMenu = !g.inMenu
			if g.inMenu {
				g.paused = true
				g.bgPlayer.Pause()
			} else {
				g.paused = false
				g.bgPlayer.Play()
			}
		}
	}

	if g.inTitle {
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeySpace) {
			g.reset()
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyS) {
			g.inMenu = true
			g.inTitle = false
		}
		return nil
	}

	if g.inMenu {
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) || inpututil.IsKeyJustPressed(ebiten.KeyW) {
			g.menuOption = (g.menuOption - 1 + 4) % 4
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) || inpututil.IsKeyJustPressed(ebiten.KeyS) {
			g.menuOption = (g.menuOption + 1) % 4
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeySpace) {
			switch g.menuOption {
			case 0: // Resume
				if !g.gameOver {
					g.inMenu = false
					g.paused = false
					g.bgPlayer.Play()
				}
			case 1: // New Game
				g.reset()
			case 2: // Reset Stats
				g.gameData = GameData{}
				g.saveGameData()
			case 3: // Back to Title
				g.inTitle = true
				g.inMenu = false
				g.paused = false
				g.bgPlayer.Pause()
			}
		}
		return nil
	}

	// Game controls
	if inpututil.IsKeyJustPressed(ebiten.KeyP) && !g.gameOver {
		g.paused = !g.paused
		if g.paused {
			g.bgPlayer.Pause()
		} else {
			g.bgPlayer.Play()
		}
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

	if g.gameOver {
		g.bgPlayer.Pause()
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
			g.reset()
		}
		return nil
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

	if g.paused {
		return nil
	}

	// Update timers and effects
	g.frame++
	g.foodPulse += 0.08
	g.headPulse += 0.1
	g.backgroundWave += 0.02
	
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
		if g.powerUp.timer <= 0 {
			g.powerUp.active = false
		}
	} else if g.frame % 300 == 0 { // Try to spawn power-up every 5 seconds
		g.placePowerUp()
	}

	g.updateParticles()

	if g.frame%g.speed != 0 {
		return nil
	}

	// Game logic
	g.dir = g.nextDir
	head := g.snake[0]
	newHead := Point{(head.X + g.dir.X + gridW) % gridW, (head.Y + g.dir.Y + gridH) % gridH}

	// Check collision with snake body
	if g.invulnerable == 0 {
		for _, s := range g.snake {
			if s == newHead {
				g.gameOver = true
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
		
		// Add particles
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

func drawEnhancedCell(x, y int, c color.RGBA, screen *ebiten.Image, scale float64, g *Game, opacity float64) {
	if opacity <= 0 {
		return
	}
	
	size := float64(cellSize) * scale * g.scaleFactor
	offset := float64(cellSize) * (1-scale) / 2
	posX := (float64(x*cellSize)+offset)*g.scaleFactor
	posY := (float64(y*cellSize)+offset)*g.scaleFactor
	
	// Apply screen shake
	if g.shakeIntensity > 0 {
		posX += (g.rng.Float64() - 0.5) * g.shakeIntensity * g.scaleFactor
		posY += (g.rng.Float64() - 0.5) * g.shakeIntensity * g.scaleFactor
	}
	
	// Draw shadow first
	shadowOffset := 2.0 * g.scaleFactor
	shadowColor := color.RGBA{0, 0, 0, uint8(float64(shadowColor.A) * opacity * 0.3)}
	ebitenutil.DrawRect(screen, posX+shadowOffset, posY+shadowOffset, size, size, shadowColor)
	
	// Apply opacity to main color
	finalColor := color.RGBA{c.R, c.G, c.B, uint8(float64(c.A) * opacity)}
	
	// Draw main cell with rounded corners effect
	// cornerRadius := size * 0.15
	
	// Main rectangle
	ebitenutil.DrawRect(screen, posX, posY, size, size, finalColor)
	
	// Highlight effect for certain cells
	if scale > 0.95 {
		highlightColor := color.RGBA{
			uint8(math.Min(255, float64(c.R)+50)),
			uint8(math.Min(255, float64(c.G)+50)),
			uint8(math.Min(255, float64(c.B)+50)),
			uint8(float64(c.A) * opacity * 0.6),
		}
		highlightSize := size * 0.3
		highlightOffset := size * 0.1
		ebitenutil.DrawRect(screen, posX+highlightOffset, posY+highlightOffset, highlightSize, highlightSize, highlightColor)
	}
}

func (g *Game) drawParticles(screen *ebiten.Image) {
	for _, p := range g.particles {
		if p.life > 0 {
			alpha := uint8(float64(p.color.A) * p.life)
			particleColor := color.RGBA{p.color.R, p.color.G, p.color.B, alpha}
			
			x := p.pos.X * g.scaleFactor
			y := p.pos.Y * g.scaleFactor
			size := p.size * g.scaleFactor
			
			// Apply screen shake to particles too
			if g.shakeIntensity > 0 {
				x += (g.rng.Float64() - 0.5) * g.shakeIntensity * 0.5 * g.scaleFactor
				y += (g.rng.Float64() - 0.5) * g.shakeIntensity * 0.5 * g.scaleFactor
			}
			
			ebitenutil.DrawRect(screen, x-size/2, y-size/2, size, size, particleColor)
		}
	}
}

func (g *Game) drawAnimatedBackground(screen *ebiten.Image) {
	// Animated grid with wave effect
	waveIntensity := 0.3
	for x := 0; x < gridW; x++ {
		for y := 0; y < gridH; y++ {
			wave := math.Sin(g.backgroundWave + float64(x+y)*0.1) * waveIntensity
			brightness := 30 + int(wave*10)
			if brightness < 0 { brightness = 0 }
			if brightness > 60 { brightness = 60 }
			
			waveColor := color.RGBA{uint8(brightness), uint8(brightness), uint8(brightness + 10), 255}
			drawEnhancedCell(x, y, waveColor, screen, 0.05, g, 0.3)
		}
	}
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(bgColor)
	
	if g.inTitle {
		g.drawTitleScreen(screen)
		return
	}
	
	if g.inMenu {
		g.drawMenuScreen(screen)
		return
	}

	// Draw animated background
	g.drawAnimatedBackground(screen)

	// Draw grid lines with subtle glow
	for x := 0; x <= gridW; x++ {
		posX := float64(x*cellSize) * g.scaleFactor
		ebitenutil.DrawRect(screen, posX, 0, 1*g.scaleFactor, float64(gridH*cellSize)*g.scaleFactor, gridColor)
	}
	for y := 0; y <= gridH; y++ {
		posY := float64(y*cellSize) * g.scaleFactor
		ebitenutil.DrawRect(screen, 0, posY, float64(gridW*cellSize)*g.scaleFactor, 1*g.scaleFactor, gridColor)
	}

	// Draw power-up
	if g.powerUp.active {
		pulse := 0.8 + 0.2*math.Sin(g.powerUp.pulse)
		var powerColor color.RGBA
		switch g.powerUp.type_ {
		case 0: powerColor = bonusColor
		case 1: powerColor = color.RGBA{100, 255, 100, 255}
		case 2: powerColor = color.RGBA{100, 100, 255, 255}
		}
		drawEnhancedCell(g.powerUp.pos.X, g.powerUp.pos.Y, powerColor, screen, pulse, g, 1.0)
	}

	// Draw food with enhanced pulsing
	pulse := 0.85 + 0.15*math.Sin(g.foodPulse)
	currentFoodColor := foodColor
	if g.combo > 0 {
		// Rainbow effect for combo food
		hue := math.Mod(g.foodPulse*2, 2*math.Pi)
		currentFoodColor = color.RGBA{
			uint8(127 + 127*math.Sin(hue)),
			uint8(127 + 127*math.Sin(hue+2*math.Pi/3)),
			uint8(127 + 127*math.Sin(hue+4*math.Pi/3)),
			255,
		}
	}
	drawEnhancedCell(g.food.X, g.food.Y, currentFoodColor, screen, pulse, g, 1.0)

	// Draw snake with trail effect
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
				// Flashing invulnerability
				if (g.frame/5)%2 == 0 {
					currentHeadColor = color.RGBA{255, 255, 100, 255}
				}
			} else if g.speedBoostTime > 0 {
				currentHeadColor = color.RGBA{255, 150, 100, 255}
			} else if g.slowMotionTime > 0 {
				currentHeadColor = color.RGBA{100, 150, 255, 255}
			}
			
			drawEnhancedCell(s.X, s.Y, currentHeadColor, screen, headScale, g, opacity)
		} else {
			// Body with gradient effect
			bodyScale := 0.9 - float64(i)*0.01
			if bodyScale < 0.5 { bodyScale = 0.5 }
			
			// Gradient body color
			factor := float64(i) / float64(len(g.snake))
			currentBodyColor := color.RGBA{
				uint8(float64(bodyColor.R) * (1 - factor*0.3)),
				uint8(float64(bodyColor.G) * (1 - factor*0.3)),
				uint8(float64(bodyColor.B) * (1 - factor*0.3)),
				bodyColor.A,
			}
			
			drawEnhancedCell(s.X, s.Y, currentBodyColor, screen, bodyScale, g, opacity)
		}
	}

	// Draw particles
	g.drawParticles(screen)

	// Enhanced HUD
	g.drawHUD(screen)
}

func (g *Game) drawTitleScreen(screen *ebiten.Image) {
	screenWidth := float64(gridW * cellSize)
	screenHeight := float64(gridH * cellSize)
	
	// Animated title background
	for i := 0; i < 20; i++ {
		x := int(float64(i) + math.Sin(g.backgroundWave+float64(i)*0.5)*2) % gridW
		y := int(float64(i*2) + math.Cos(g.backgroundWave+float64(i)*0.3)*1) % gridH
		if x >= 0 && y >= 0 && x < gridW && y < gridH {
			alpha := 0.3 + 0.2*math.Sin(g.backgroundWave+float64(i)*0.2)
			color := color.RGBA{50, 100, 50, uint8(alpha * 255)}
			drawEnhancedCell(x, y, color, screen, 0.6, g, alpha)
		}
	}
	
	lines := []string{
		"🐍 ENHANCED SNAKE 🐍",
		"",
		"🎮 Features:",
		"• Combo System & Power-ups",
		"• Particle Effects & Smooth Animations",
		"• Enhanced Audio & Visual Effects",
		"• Statistics Tracking",
		"• Dynamic Difficulty",
		"",
		"🎯 Controls:",
		"Arrow Keys/WASD: Move",
		"P: Pause | F: Fullscreen | Esc: Menu",
		"+/-: Speed Control",
		"",
		"🏆 High Score: " + fmt.Sprintf("%d", g.gameData.HighScore),
		fmt.Sprintf("Games Played: %d | Best Combo: %d", g.gameData.TotalGames, g.gameData.BestCombo),
		"",
		"Press ENTER or SPACE to start!",
		"Press S for Statistics",
		"",
		"Enhanced by Claude AI",
	}
	
	lineHeight := 18.0 * g.scaleFactor
	totalHeight := float64(len(lines)) * lineHeight
	startY := (screenHeight - totalHeight) / 2
	
	for i, line := range lines {
		if line == "" {
			continue
		}
		
		approxWidth := float64(len(line)) * 7 * g.scaleFactor
		x := (screenWidth - approxWidth) / 2
		y := startY + float64(i)*lineHeight
		
		// Add glow effect to title
		if i == 0 {
			for dx := -1; dx <= 1; dx++ {
				for dy := -1; dy <= 1; dy++ {
					if dx != 0 || dy != 0 {
						ebitenutil.DebugPrintAt(screen, line, int(x)+dx*2, int(y)+dy*2)
					}
				}
			}
		}
		
		ebitenutil.DebugPrintAt(screen, line, int(x), int(y))
	}
}

func (g *Game) drawMenuScreen(screen *ebiten.Image) {
	// Semi-transparent overlay
	overlay := ebiten.NewImage(int(float64(gridW*cellSize)*g.scaleFactor), int(float64(gridH*cellSize)*g.scaleFactor))
	overlay.Fill(color.RGBA{0, 0, 0, 150})
	screen.DrawImage(overlay, nil)
	
	screenWidth := float64(gridW * cellSize)
	screenHeight := float64(gridH * cellSize)
	
	menuItems := []string{
		"Resume Game",
		"New Game",
		"Reset Statistics",
		"Back to Title",
	}
	
	if g.gameOver {
		menuItems[0] = "Game Over!"
	}
	
	lineHeight := 30.0 * g.scaleFactor
	totalHeight := float64(len(menuItems)) * lineHeight
	startY := (screenHeight - totalHeight) / 2
	
	// Menu title
	title := "=== PAUSE MENU ==="
	if g.gameOver {
		title = "=== GAME OVER ==="
	}
	titleWidth := float64(len(title)) * 8 * g.scaleFactor
	titleX := (screenWidth - titleWidth) / 2
	titleY := startY - 50*g.scaleFactor
	ebitenutil.DebugPrintAt(screen, title, int(titleX), int(titleY))
	
	for i, item := range menuItems {
		approxWidth := float64(len(item)) * 8 * g.scaleFactor
		x := (screenWidth - approxWidth) / 2
		y := startY + float64(i)*lineHeight
		
		// Highlight selected option
		if i == g.menuOption {
			prefix := "► "
			suffix := " ◄"
			fullText := prefix + item + suffix
			fullWidth := float64(len(fullText)) * 8 * g.scaleFactor
			fullX := (screenWidth - fullWidth) / 2
			ebitenutil.DebugPrintAt(screen, fullText, int(fullX), int(y))
		} else {
			ebitenutil.DebugPrintAt(screen, item, int(x), int(y))
		}
	}
	
	// Show current game stats if in game
	if !g.gameOver && !g.inTitle {
		statsY := startY + float64(len(menuItems))*lineHeight + 40*g.scaleFactor
		stats := []string{
			fmt.Sprintf("Current Score: %d", g.score),
			fmt.Sprintf("Current Combo: %d (Max: %d)", g.combo, g.maxCombo),
			fmt.Sprintf("Snake Length: %d", len(g.snake)),
		}
		
		for i, stat := range stats {
			statWidth := float64(len(stat)) * 8 * g.scaleFactor
			statX := (screenWidth - statWidth) / 2
			statY := statsY + float64(i)*20*g.scaleFactor
			ebitenutil.DebugPrintAt(screen, stat, int(statX), int(statY))
		}
	}
}

func (g *Game) drawHUD(screen *ebiten.Image) {
	padding := 10.0 * g.scaleFactor
	lineHeight := 16.0 * g.scaleFactor
	
	// Main HUD
	lines := []string{
		fmt.Sprintf("Score: %d | High: %d | Speed: %d/%d", g.score, g.gameData.HighScore, maxSpeed-g.baseSpeed+minSpeed, maxSpeed-minSpeed+1),
		fmt.Sprintf("Length: %d | Combo: %d (Max: %d)", len(g.snake), g.combo, g.maxCombo),
	}
	
	// Status effects
	var effects []string
	if g.speedBoostTime > 0 {
		effects = append(effects, fmt.Sprintf("SPEED BOOST: %ds", g.speedBoostTime/60+1))
	}
	if g.slowMotionTime > 0 {
		effects = append(effects, fmt.Sprintf("SLOW MOTION: %ds", g.slowMotionTime/60+1))
	}
	if g.invulnerable > 0 {
		effects = append(effects, fmt.Sprintf("INVULNERABLE: %ds", g.invulnerable/60+1))
	}
	if g.paused {
		effects = append(effects, "PAUSED - Press P to Resume")
	}
	
	// Power-up indicator
	if g.powerUp.active {
		powerUpNames := []string{"BONUS POINTS", "SPEED BOOST", "INVULNERABILITY"}
		effects = append(effects, fmt.Sprintf("Power-up: %s (%ds)", powerUpNames[g.powerUp.type_], g.powerUp.timer/60+1))
	}
	
	lines = append(lines, effects...)
	
	// Controls hint
	if g.frame < 300 { // Show for first 5 seconds
		lines = append(lines, "ESC: Menu | +/-: Speed | P: Pause | F: Fullscreen")
	}
	
	for i, line := range lines {
		y := padding + float64(i)*lineHeight
		ebitenutil.DebugPrintAt(screen, line, int(padding), int(y))
	}
	
	// Progress bars for effects
	barY := padding + float64(len(lines))*lineHeight + 5*g.scaleFactor
	barWidth := 200.0 * g.scaleFactor
	barHeight := 8.0 * g.scaleFactor
	
	if g.speedBoostTime > 0 {
		progress := float64(g.speedBoostTime) / 300.0
		ebitenutil.DrawRect(screen, padding, barY, barWidth, barHeight, color.RGBA{50, 50, 50, 255})
		ebitenutil.DrawRect(screen, padding, barY, barWidth*progress, barHeight, color.RGBA{255, 150, 100, 255})
		barY += barHeight + 2*g.scaleFactor
	}
	
	if g.invulnerable > 0 {
		progress := float64(g.invulnerable) / 180.0
		ebitenutil.DrawRect(screen, padding, barY, barWidth, barHeight, color.RGBA{50, 50, 50, 255})
		ebitenutil.DrawRect(screen, padding, barY, barWidth*progress, barHeight, color.RGBA{100, 150, 255, 255})
	}
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	g.isFullscreen = ebiten.IsWindowMaximized()
	scaleX := float64(outsideWidth) / float64(gridW*cellSize)
	scaleY := float64(outsideHeight) / float64(gridH*cellSize)
	g.scaleFactor = math.Min(scaleX, scaleY)
	
	// Ensure minimum scale
	if g.scaleFactor < 0.5 {
		g.scaleFactor = 0.5
	}
	
	return int(float64(gridW*cellSize) * g.scaleFactor), int(float64(gridH*cellSize) * g.scaleFactor)
}

func main() {
	ebiten.SetWindowSize(1280, 720)
	ebiten.SetWindowTitle("Enhanced Snake — Go + Ebiten")
	ebiten.SetWindowResizable(true)
	ebiten.SetWindowSizeLimits(640, 480, -1, -1)
	
	if err := ebiten.RunGame(NewGame()); err != nil {
		log.Fatal(err)
	}
}