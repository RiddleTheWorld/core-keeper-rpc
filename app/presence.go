package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"
)

// --- game status

// Status is what the in-game mod last reported.
type Status struct {
	Kind      string `json:"kind"` // "none", "menu", "game"
	World     string `json:"world"`
	Character string `json:"character"`
	Biome     string `json:"biome"`
	Place     string `json:"place"`
	Boss      string `json:"boss"`
	HP        int    `json:"hp"`
	MaxHP     int    `json:"maxHp"`
	Party     int    `json:"party"`
	Safe      bool   `json:"safe"`
	Food      string `json:"food"` // hunger 0-100, "" if unknown
	Mana      string `json:"mana"`
	MaxMana   string `json:"maxMana"`
	Held      string `json:"held"` // item in hand
	X         string `json:"x"`    // tile coordinates; the Core is at 0,0
	Y         string `json:"y"`
}

const modPrefix = "[CKRP]|v1|"

// parseLine reads one [CKRP] line from the mod. Kind "quit" means the game closed.
func parseLine(line string) (Status, bool) {
	rest, ok := strings.CutPrefix(strings.TrimRight(line, "\r\n "), modPrefix)
	if !ok {
		return Status{}, false
	}
	f := strings.Split(rest, "|")
	switch f[0] {
	case "menu", "loaded":
		return Status{Kind: "menu"}, true
	case "quit":
		return Status{Kind: "quit"}, true
	case "game":
		if len(f) < 11 {
			return Status{}, false
		}
		atoi := func(s string) int { n, _ := strconv.Atoi(s); return n }
		s := Status{
			Kind: "game", World: f[1], Character: f[2], Biome: f[3], Place: f[4], Boss: f[5],
			HP: atoi(f[6]), MaxHP: atoi(f[7]), Party: max(1, atoi(f[8])), Safe: f[9] == "1",
		}
		if len(f) >= 17 { // newer mod: extra stats
			s.Food, s.Mana, s.MaxMana, s.Held, s.X, s.Y = f[11], f[12], f[13], f[14], f[15], f[16]
		}
		return s, true
	}
	return Status{}, false
}

// parseServerName extracts the world name from "Connected to server <name> (<guid>)".
func parseServerName(line string) string {
	rest, ok := strings.CutPrefix(strings.TrimRight(line, "\r\n "), "Connected to server ")
	if !ok {
		return ""
	}
	if i := strings.LastIndex(rest, " ("); i > 0 {
		return strings.TrimSpace(rest[:i])
	}
	return ""
}

// Situation picks which template set applies.
func Situation(s Status) string {
	switch {
	case s.Kind != "game":
		return "menu"
	case s.MaxHP > 0 && s.HP <= 0:
		return "dead"
	case s.Boss != "":
		return "boss"
	case s.Place == "Home Base" || s.Safe:
		return "base"
	case s.Place != "":
		return "dungeon"
	default:
		return "exploring"
	}
}

// --- templates

func tokens(s Status) map[string]string {
	party := "Solo"
	if s.Party > 1 {
		party = fmt.Sprintf("Party of %d", s.Party)
	}
	hp, maxHP := "", ""
	if s.MaxHP > 0 {
		hp, maxHP = strconv.Itoa(max(0, s.HP)), strconv.Itoa(s.MaxHP)
	}
	return map[string]string{
		"{biome}": s.Biome, "{boss}": s.Boss, "{place}": s.Place, "{world}": s.World,
		"{character}": s.Character, "{hp}": hp, "{maxhp}": maxHP,
		"{party}": strconv.Itoa(max(1, s.Party)), "{partytext}": party,
		"{food}": s.Food, "{mana}": s.Mana, "{maxmana}": s.MaxMana, "{held}": s.Held,
		"{x}": s.X, "{y}": s.Y, "{distance}": distance(s),
	}
}

// distance is how many tiles you are from the Core (the world centre).
func distance(s Status) string {
	x, errX := strconv.Atoi(s.X)
	y, errY := strconv.Atoi(s.Y)
	if errX != nil || errY != nil {
		return ""
	}
	return strconv.Itoa(int(math.Round(math.Sqrt(float64(x*x + y*y)))))
}

// Render fills tokens and drops the " · " segments whose tokens were all empty,
// e.g. "❤ / · Solo" never appears. A value is inserted as it is and never scanned
// again, so a world or character name containing braces stays readable.
func Render(tmpl string, s Status) string {
	t := tokens(s)
	pairs := make([]string, 0, 2*len(t))
	for k, v := range t {
		pairs = append(pairs, k, v)
	}
	replace := strings.NewReplacer(pairs...)
	var out []string
	for _, seg := range strings.Split(tmpl, "·") {
		hadToken, allEmpty := false, true
		for k, v := range t {
			if strings.Contains(seg, k) {
				hadToken = true
				if v != "" {
					allEmpty = false
				}
			}
		}
		filled := strings.TrimSpace(replace.Replace(seg))
		if filled == "" || (hadToken && allEmpty) {
			continue
		}
		out = append(out, filled)
	}
	return fit(strings.Join(out, " · "))
}

// fit enforces Discord's 128-character limit.
func fit(s string) string {
	if utf8.RuneCountInString(s) <= 128 {
		return s
	}
	return string([]rune(s)[:128])
}

// --- Discord IPC

type discordClient struct {
	pipe  *os.File
	nonce int
}

func frame(op uint32, payload any) ([]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	binary.Write(&b, binary.LittleEndian, op)
	binary.Write(&b, binary.LittleEndian, uint32(len(body)))
	b.Write(body)
	return b.Bytes(), nil
}

func (d *discordClient) exchange(op uint32, payload any) (map[string]any, error) {
	data, err := frame(op, payload)
	if err != nil {
		return nil, err
	}
	if _, err := d.pipe.Write(data); err != nil {
		return nil, err
	}
	var hdr [8]byte
	if _, err := io.ReadFull(d.pipe, hdr[:]); err != nil {
		return nil, err
	}
	n := binary.LittleEndian.Uint32(hdr[4:])
	if n > 1<<20 {
		return nil, errors.New("reply too large")
	}
	body := make([]byte, n)
	if _, err := io.ReadFull(d.pipe, body); err != nil {
		return nil, err
	}
	var reply map[string]any
	return reply, json.Unmarshal(body, &reply)
}

func (d *discordClient) connected() bool { return d.pipe != nil }

func (d *discordClient) connect() error {
	lastErr := errors.New("Discord desktop app not found (is it running?)")
	for i := 0; i < 10; i++ {
		f, err := os.OpenFile(fmt.Sprintf(`\\.\pipe\discord-ipc-%d`, i), os.O_RDWR, 0)
		if err != nil {
			continue
		}
		d.pipe = f
		reply, err := d.exchange(0, map[string]any{"v": 1, "client_id": clientID})
		if err == nil && reply["evt"] == "READY" {
			return nil
		}
		f.Close()
		d.pipe = nil
		if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("handshake rejected")
		}
	}
	return lastErr
}

// setActivity sends an activity, or clears it when activity is nil.
func (d *discordClient) setActivity(activity map[string]any) error {
	if d.pipe == nil {
		return errors.New("not connected")
	}
	d.nonce++
	args := map[string]any{"pid": os.Getpid()}
	if activity != nil {
		args["activity"] = activity
	}
	reply, err := d.exchange(1, map[string]any{"cmd": "SET_ACTIVITY", "args": args, "nonce": strconv.Itoa(d.nonce)})
	if err != nil {
		d.pipe.Close()
		d.pipe = nil
		return err
	}
	if reply["evt"] == "ERROR" {
		return fmt.Errorf("Discord refused it: %v", reply["data"])
	}
	return nil
}
