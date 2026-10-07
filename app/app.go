package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	clientID     = "1124351749550002268" // Core Keeper's official Discord application
	largeImage   = "https://cdn.cloudflare.steamstatic.com/steam/apps/1621690/header.jpg"
	poll         = 1 * time.Second
	minGap       = 3 * time.Second // fastest allowed spacing between Discord updates
	burstLimit   = 5               // Discord accepts 5 activity updates per 20 s and silently drops the rest
	burstWindow  = 20 * time.Second
	gameTimeout  = 75 * time.Second // mod heartbeat is 30 s
	reconnectGap = 15 * time.Second
)

// LiveState is what the UI shows in its header, preview and token readout.
type LiveState struct {
	Game      string            `json:"game"` // "closed", "menu", "playing"
	Discord   string            `json:"discord"`
	Situation string            `json:"situation"`
	Details   string            `json:"details"`
	State     string            `json:"state"`
	Tokens    map[string]string `json:"tokens"` // what each "{token}" means right now
}

type App struct {
	ctx context.Context
	mu  sync.Mutex // guards everything below except discord
	dmu sync.Mutex // guards discord; held during pipe I/O, so the UI never waits on Discord
	cfg Config

	logPath     string
	offset      int64
	partial     []byte
	status      Status
	serverWorld string
	startedAt   int64
	wasInGame   bool

	discord    discordClient
	discordMsg string
	shown      string
	sent       []time.Time // when recent updates went out, for the burst limit
	lastTry    time.Time
	live       LiveState
	forcePush  bool
}

func NewApp() *App {
	home, _ := os.UserHomeDir()
	return &App{
		cfg:       LoadConfig(),
		logPath:   filepath.Join(home, `AppData\LocalLow\Pugstorm\Core Keeper\Player.log`),
		status:    Status{Kind: "none"},
		startedAt: time.Now().Unix(),
		live:      LiveState{Game: "closed", Tokens: map[string]string{}},
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	go a.loop()
}

// --- bound to the UI

func (a *App) GetConfig() Config {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.cfg
}

func (a *App) DefaultConfig() Config { return DefaultConfig() }

func (a *App) SaveConfig(cfg Config) error {
	if err := SaveConfig(cfg); err != nil {
		return err
	}
	a.mu.Lock()
	a.cfg = cfg
	a.forcePush = true
	a.mu.Unlock()
	return nil
}

func (a *App) GetLive() LiveState {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.live
}

// Quit clears the Discord status and exits (closing the window only hides it).
func (a *App) Quit() {
	a.dmu.Lock()
	if a.discord.connected() {
		a.discord.setActivity(nil)
	}
	a.dmu.Unlock()
	runtime.Quit(a.ctx)
}

// Preview renders a situation with the live status if it matches, else sample data.
func (a *App) Preview(situation string, lines Lines) Lines {
	a.mu.Lock()
	s := a.status
	a.mu.Unlock()
	if s.Kind != "game" || Situation(s) != situation {
		s = sampleStatus(situation)
	}
	return Lines{Details: Render(lines.Details, s), State: Render(lines.State, s)}
}

func sampleStatus(situation string) Status {
	s := Status{Kind: "game", World: "wddw", Character: "Ruby", Biome: "Forgotten Ruins", HP: 245, MaxHP: 300, Party: 2,
		Food: "87", Mana: "40", MaxMana: "60", Held: "Tin Pickaxe", X: "-130", Y: "214"}
	switch situation {
	case "menu":
		return Status{Kind: "menu"}
	case "base":
		s.Safe, s.Place, s.Biome = true, "Home Base", "Dirt Biome"
	case "dungeon":
		s.Place = "Ancient City"
	case "boss":
		s.Boss = "Malugaz the Corrupted"
	case "dead":
		s.HP = 0
	}
	return s
}

// --- background loop

func (a *App) loop() {
	for {
		a.tick()
		time.Sleep(poll)
	}
}

func (a *App) tick() {
	a.readLog()

	a.mu.Lock()
	alive := false
	if fi, err := os.Stat(a.logPath); err == nil {
		alive = time.Since(fi.ModTime()) < gameTimeout && a.status.Kind != "quit" && a.status.Kind != "none"
	}
	inGame := alive && a.status.Kind == "game"
	if inGame != a.wasInGame {
		a.startedAt, a.wasInGame = time.Now().Unix(), inGame
	}

	s := a.status
	if s.World == "" {
		s.World = a.serverWorld
	}
	sit := Situation(s)
	lines := a.cfg.Situations[sit]
	details, state := Render(lines.Details, s), Render(lines.State, s)

	var activity map[string]any
	if alive && a.cfg.Enabled {
		activity = map[string]any{"details": orDefault(details, "Playing")}
		if len([]rune(state)) >= 2 {
			activity["state"] = state
		}
		if a.cfg.ShowTimer {
			activity["timestamps"] = map[string]any{"start": a.startedAt}
		}
		if a.cfg.ShowImage {
			activity["assets"] = map[string]any{"large_image": largeImage, "large_text": orDefault(a.cfg.ImageText, "Core Keeper")}
		}
	}

	key := ""
	if activity != nil {
		key = sit + "|" + details + "|" + state
	}
	due := (key != a.shown || a.forcePush) && a.canSend(time.Now())
	a.mu.Unlock()
	if due {
		a.push(activity, key)
	}
	a.mu.Lock()
	defer a.mu.Unlock()

	game := "closed"
	if inGame {
		game = "playing"
	} else if alive {
		game = "menu"
	}
	a.live = LiveState{Game: game, Discord: a.discordMsg, Situation: sit, Tokens: tokens(s), Details: details, State: state}
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "live", a.live)
	}
}

// push sends the activity to Discord. Called without a.mu held: pipe I/O can block.
func (a *App) push(activity map[string]any, key string) {
	a.dmu.Lock()
	defer a.dmu.Unlock()
	fail := func(err error) {
		a.mu.Lock()
		a.discordMsg = err.Error()
		a.mu.Unlock()
	}
	if !a.discord.connected() {
		if activity == nil { // nothing to clear while Discord is away
			a.mu.Lock()
			a.shown, a.forcePush = key, false
			a.mu.Unlock()
			return
		}
		a.mu.Lock()
		canTry := time.Since(a.lastTry) >= reconnectGap
		if canTry {
			a.lastTry = time.Now()
		}
		a.mu.Unlock()
		if !canTry {
			return
		}
		if err := a.discord.connect(); err != nil {
			fail(err)
			return
		}
	}
	a.mu.Lock()
	a.sent = append(a.sent, time.Now())
	a.mu.Unlock()
	if err := a.discord.setActivity(activity); err != nil {
		fail(err)
		return
	}
	a.mu.Lock()
	a.discordMsg, a.shown, a.forcePush = "connected", key, false
	a.mu.Unlock()
}

// canSend reports whether an update may go out now: the player's chosen spacing
// (at least minGap) has passed, and fewer than burstLimit went out in the last
// burstWindow. A change that has to wait is sent on a later tick, so the newest
// state always reaches Discord. Call with a.mu held.
func (a *App) canSend(now time.Time) bool {
	recent := a.sent[:0]
	for _, t := range a.sent {
		if now.Sub(t) < burstWindow {
			recent = append(recent, t)
		}
	}
	a.sent = recent
	if len(recent) >= burstLimit {
		return false
	}
	gap := max(minGap, time.Duration(a.cfg.UpdateSecs)*time.Second)
	return len(recent) == 0 || now.Sub(recent[len(recent)-1]) >= gap
}

// readLog reads bytes appended to Player.log since the last call.
func (a *App) readLog() {
	f, err := os.Open(a.logPath)
	if err != nil {
		return
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return
	}
	a.mu.Lock()
	if fi.Size() < a.offset { // game relaunched: log truncated
		a.offset, a.partial = 0, nil
	}
	offset := a.offset
	a.mu.Unlock()
	if fi.Size() == offset {
		return
	}
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return
	}
	buf, err := io.ReadAll(io.LimitReader(f, fi.Size()-offset))
	if err != nil {
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	a.offset = offset + int64(len(buf))
	a.partial = append(a.partial, buf...)
	for {
		i := bytes.IndexByte(a.partial, '\n')
		if i < 0 {
			if len(a.partial) > 64<<10 { // runaway line with no newline: drop it
				a.partial = nil
			}
			break
		}
		line := string(a.partial[:i])
		a.partial = a.partial[i+1:]
		if s, ok := parseLine(line); ok {
			a.status = s
		} else if name := parseServerName(line); name != "" {
			a.serverWorld = name
		} else if strings.HasPrefix(line, "starting a new world") {
			a.serverWorld = ""
		}
	}
}

func orDefault(s, d string) string {
	if len([]rune(strings.TrimSpace(s))) < 2 {
		return d
	}
	return s
}
