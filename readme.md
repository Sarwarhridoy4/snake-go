# Snake Game

Welcome to **Snake**, a classic arcade-style game developed by **Sarwar Hossain** using **Go** and the **Ebiten** game library. Guide your snake to eat food, grow longer, and score points while avoiding collisions with yourself.

This README explains how to set up, build, and play the game on Windows and Linux.

---

## Table of Contents

- [Game Overview](#game-overview)
- [How to Play](#how-to-play)
- [Controls](#controls)
- [Setup and Build Instructions](#setup-and-build-instructions)
- [Game Features](#game-features)
- [Developer](#developer)

---

## Game Overview

In Snake, you control a snake that moves around a grid, eating food to grow longer and increase your score. Each food item eaten adds to your score and may trigger a combo for bonus points.

**Avoid colliding with your own body, or it's game over!**

The game includes:

- High-score system
- Pause functionality
- Adjustable speed for a tailored experience

---

## How to Play

### Start the Game

- Launch the game to see the **title screen**, with text centered in the window.
- Press **Enter** or **Space** to begin playing.

### Objective

- Navigate the snake to eat **red food items** (or orange during combos) to grow longer and increase your score.
- Each food item adds **1 point** plus a bonus based on your combo streak (e.g., Combo: x3 adds 1 + 3/2 = 2 points).
- Avoid hitting your own body — this ends the game.
- Try to beat your **high score**, saved automatically to `snake_highscore.json`.

### Game States

- **Title Screen:** Displays game title, instructions, and developer credit. Press **Enter** or **Space** to start.
- **Gameplay:** Move the snake to eat food and grow. The HUD in the top-left corner shows your score, high score, speed, and controls.
- **Paused:** Press **P** to pause/resume. HUD displays `"Paused - Press P to Resume."`
- **Game Over:** If the snake collides with itself, the game ends. HUD shows final score and prompts to press **Enter** or **R** to restart.

---

## Controls

### Movement

- **Arrow Keys or WASD:**
  - **Up Arrow / W:** Move up
  - **Down Arrow / S:** Move down
  - **Left Arrow / A:** Move left
  - **Right Arrow / D:** Move right

> The snake cannot reverse directly into itself.

### Game Controls

- **P:** Pause/resume
- **Enter / R:** Restart after game over
- **Enter / Space:** Start game from title screen
- **+ / =:** Increase speed (up to a maximum)
- **-:** Decrease speed (down to a minimum)

### Window Controls

- **F:** Maximize window (full-screen)
- **Esc:** Restore default window size (1280x720)
- Window is **resizable** by dragging edges

---

## Setup and Build Instructions

### Prerequisites

- **Go:** Install version 1.16 or later from [golang.org](https://golang.org/dl/)
- **Git:** Required to fetch dependencies

### Clone the Repository

```bash
git clone <repository-url>
cd snake
```

### Initialize Go Module

```bash
go mod init snake
```

This creates a `go.mod` file in your project directory.

### Install Ebiten Package

```bash
go get github.com/hajimehoshi/ebiten/v2
```

This adds Ebiten as a dependency to your `go.mod` and `go.sum`.

### Linux Dependencies

For Linux, you may need graphics and audio libraries:

```bash
sudo apt-get install libgl1-mesa-dev libxrandr-dev libxinerama-dev libxi-dev libasound2-dev
```

_(Adjust for your distribution: yum, pacman, etc.)_

---

### Build for Windows

```bash
GOOS=windows GOARCH=amd64 go build -o snake-windows.exe main.go
```

- Outputs `snake-windows.exe`.
- Use `GOARCH=386` for 32-bit Windows if needed.
- Copy to Windows machine if built on Linux.
- Run:

```bash
snake-windows.exe
```

- Ensure `snake_highscore.json` is in the same directory for high-score persistence.

### Build for Linux

```bash
GOOS=linux GOARCH=amd64 go build -o snake-linux main.go
```

- Outputs `snake-linux`.
- Copy to Linux machine if built on Windows.
- Make executable:

```bash
chmod +x snake-linux
```

- Run:

```bash
./snake-linux
```

- Ensure `snake_highscore.json` is in the same directory.

### Run Without Building

```bash
go run main.go
```

Opens the game in a **1280x720 window** titled `Snake — Go + Ebiten`.

**Notes:**

- Cross-Compilation: Build for Windows from Linux or vice versa using `GOOS` and `GOARCH`.
- Dependencies: Ensure your system has OpenGL (Windows/Linux) and ALSA (Linux) for rendering and audio.
- High Score: Saved to `snake_highscore.json` in the executable’s directory.

---

## Game Features

- **Responsive Design:** Scales dynamically to fit any window size.
- **Clean Visuals:** Dark background, grid lines, uniform snake color, subtle food pulse effect.
- **Centered Title Screen:** Title, instructions, and developer credit centered horizontally and vertically.
- **Top-Left HUD:** Score, high score, speed, controls, and status messages with padding.
- **Audio Feedback:** Sounds for eating food, combo streaks, game over, and background music.
- **Combo System:** Quick successive food increases bonus points.
- **High Score Persistence:** Highest score saved to JSON file.
- **Customizable Speed:** Adjust snake's speed with + or - keys.

---

## Developer

- **Developed by:** Sarwar Hossain
- **Contact:** [sarwarhridoy4@gmail.com](mailto:sarwarhridoy4@gmail.com)

Enjoy playing **Snake**! Feedback or questions are welcome.
