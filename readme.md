Snake Game
Welcome to Snake, a classic arcade-style game developed by Sarwar Hossain using Go and the Ebiten game library. Guide your snake to eat food, grow longer, and score points while avoiding collisions with yourself. This README explains how to set up, build, and play the game on Windows and Linux.
Table of Contents

Game Overview
How to Play
Controls
Setup and Build Instructions
Game Features
Developer

Game Overview
In Snake, you control a snake that moves around a grid, eating food to grow longer and increase your score. Each food item eaten adds to your score and may trigger a combo for bonus points. Avoid colliding with your own body, or it's game over! The game includes a high-score system, pause functionality, and adjustable speed for a tailored experience.
How to Play

Start the Game:

Launch the game to see the title screen, with text centered in the window.
Press Enter or Space to begin playing.


Objective:

Navigate the snake to eat red food items (or orange during combos) to grow longer and increase your score.
Each food item adds 1 point plus a bonus based on your combo streak (e.g., Combo: x3 adds 1 + 3/2 = 2 points).
Avoid hitting your own body, which ends the game.
Try to beat your high score, saved automatically to snake_highscore.json.


Game States:

Title Screen: Displays the game title, instructions, and developer credit, centered in the window. Press Enter or Space to start.
Gameplay: Move the snake to eat food and grow. The HUD in the top-left corner (with padding) shows your score, high score, speed, and controls.
Paused: Press P to pause/resume. The HUD displays "Paused - Press P to Resume."
Game Over: If the snake collides with itself, the game ends. The HUD shows your final score and prompts to press Enter or R to restart.



Controls

Movement:

Arrow Keys or WASD:
Up Arrow or W: Move up
Down Arrow or S: Move down
Left Arrow or A: Move left
Right Arrow or D: Move right


The snake cannot reverse directly into itself (e.g., can't move up if going down).


Game Controls:

P: Pause or resume the game.
Enter or R: Restart after game over.
Enter or Space: Start the game from the title screen.
+ or = (equal): Increase speed (faster movement, up to a maximum).
- (minus): Decrease speed (slower movement, down to a minimum).


Window Controls:

F: Maximize the window (treated as full-screen).
Esc: Restore the window to its default size (1280x720).
The window is resizable by dragging its edges.



Setup and Build Instructions
Prerequisites

Go: Install Go (version 1.16 or later) from https://golang.org/dl/.
Git: Required to fetch dependencies.
Ebiten Dependencies: Install the Ebiten library with:go get github.com/hajimehoshi/ebiten/v2


Linux Dependencies: For Linux, you may need graphics and audio libraries:sudo apt-get install libgl1-mesa-dev libxrandr-dev libxinerama-dev libxi-dev libasound2-dev

(Adjust for your distribution, e.g., yum or pacman.)

Clone the Repository
git clone <repository-url>
cd snake

Build for Windows

Run the following command to build a Windows executable:
GOOS=windows GOARCH=amd64 go build -o snake-windows.exe main.go


Outputs snake-windows.exe.
Use GOARCH=386 for 32-bit Windows if needed.


Copy snake-windows.exe to a Windows machine (if built on Linux).

Run the executable:
snake-windows.exe


Ensure snake_highscore.json is in the same directory for high-score persistence.


Build for Linux

Run the following command to build a Linux executable:
GOOS=linux GOARCH=amd64 go build -o snake-linux main.go


Outputs snake-linux.


Copy snake-linux to a Linux machine (if built on Windows).

Make the executable runnable:
chmod +x snake-linux


Run the executable:
./snake-linux


Ensure snake_highscore.json is in the same directory for high-score persistence.


Run Without Building
To run the game directly without creating an executable:
go run main.go


This opens the game in a 1280x720 window titled "Snake — Go + Ebiten."

Notes

Cross-Compilation: You can build for Windows from Linux or vice versa using the GOOS and GOARCH environment variables.
Dependencies: Ensure your system has OpenGL (Windows/Linux) and ALSA (Linux) for rendering and audio. Most systems have these pre-installed.
High Score: The game saves high scores to snake_highscore.json in the executable’s directory.

Game Features

Responsive Design: The game scales dynamically to fit any window size, whether resized or maximized.
Clean Visuals: Features a solid dark background, grid lines, and a uniformly colored snake with a distinct head and subtle food pulse effect.
Centered Title Screen: The welcome screen text, including game title, instructions, and developer credit, is centered horizontally and vertically.
Top-Left HUD: Score, high score, speed, controls, and status messages are displayed in the top-left corner with padding.
Audio Feedback: Includes sound effects for eating food, combo streaks, game over, and background music.
Combo System: Eating food in quick succession builds a combo, increasing your score with bonus points.
High Score Persistence: Your highest score is saved to a JSON file and displayed in the HUD.
Customizable Speed: Adjust the snake's speed with + or - keys for easier or more challenging gameplay.

Developer

Developed by: Sarwar Hossain
Contact: (sarwarhridoy4@gmail.com)

Enjoy playing Snake! If you have feedback or need assistance, feel free to reach out.