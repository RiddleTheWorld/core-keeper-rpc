package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseGameLine(t *testing.T) {
	s, ok := parseLine("[CKRP]|v1|game|wddw|Ruby|Forgotten Ruins||Malugaz the Corrupted|245|300|2|0|MALUGAZ")
	if !ok || s.Kind != "game" || s.Boss != "Malugaz the Corrupted" || s.HP != 245 || s.MaxHP != 300 || s.Party != 2 || s.Safe {
		t.Fatalf("bad parse: %+v", s)
	}
	if _, ok := parseLine("random line"); ok {
		t.Fatal("junk accepted")
	}
	if s, _ := parseLine("[CKRP]|v1|quit\r"); s.Kind != "quit" {
		t.Fatal("quit")
	}
}

func TestSituation(t *testing.T) {
	g := Status{Kind: "game", HP: 10, MaxHP: 100}
	cases := map[string]Status{
		"menu": {Kind: "menu"}, "exploring": g,
		"boss": {Kind: "game", HP: 1, MaxHP: 1, Boss: "X"}, "dead": {Kind: "game", HP: 0, MaxHP: 100, Boss: "X"},
		"base": {Kind: "game", Safe: true}, "dungeon": {Kind: "game", Place: "Ancient City"},
	}
	for want, s := range cases {
		if got := Situation(s); got != want {
			t.Errorf("%+v: got %s want %s", s, got, want)
		}
	}
}

func TestRenderDropsEmptySegments(t *testing.T) {
	s := Status{Kind: "game", Biome: "Clay Caves", HP: 5, MaxHP: 100, Party: 1}
	if got := Render("\u2764 {hp}/{maxhp} \u00b7 {partytext} \u00b7 World: {world}", s); got != "\u2764 5/100 \u00b7 Solo" {
		t.Errorf("got %q", got)
	}
	if got := Render("Exploring {biome}", s); got != "Exploring Clay Caves" {
		t.Errorf("got %q", got)
	}
	if got := Render("Party of {party}", Status{Party: 3}); got != "Party of 3" {
		t.Errorf("got %q", got)
	}
}

func TestServerName(t *testing.T) {
	if parseServerName("Connected to server My (Cool) World (abc)") != "My (Cool) World" {
		t.Fatal("server name")
	}
}

func TestFrame(t *testing.T) {
	b, _ := frame(1, map[string]int{"a": 1})
	if b[0] != 1 || b[4] != 7 || string(b[8:]) != `{"a":1}` {
		t.Fatalf("%v", b)
	}
}

func TestExtraStatsFields(t *testing.T) {
	s, ok := parseLine("[CKRP]|v1|game|W|Ruby|Dirt Biome|||206|206|1|0|SLIME_BIOME|87|40|60|Copper Pickaxe|-30|40")
	if !ok || s.Food != "87" || s.Held != "Copper Pickaxe" || s.X != "-30" || s.Y != "40" {
		t.Fatalf("%+v", s)
	}
	if got := Render("{held} \u00b7 {distance} from the Core \u00b7 food {food}", s); got != "Copper Pickaxe \u00b7 50 from the Core \u00b7 food 87" {
		t.Errorf("got %q", got)
	}
	old, _ := parseLine("[CKRP]|v1|game|W|R|B|||1|2|1|0|R")
	if Render("Holding {held} \u00b7 {partytext}", old) != "Solo" {
		t.Error("old mod lines should drop missing fields")
	}
}

func TestRenderKeepsValuesVerbatim(t *testing.T) {
	// A world or character name that looks like a token is inserted as it is and
	// never replaced again.
	s := Status{Kind: "game", Biome: "{hp}", World: "wddw", HP: 42, MaxHP: 100}
	if got := Render("{biome} \u00b7 {world} \u00b7 {hp}", s); got != "{hp} \u00b7 wddw \u00b7 42" {
		t.Errorf("got %q", got)
	}
}

// writeConfig writes body to a throwaway config.json the next LoadConfig reads.
func writeConfig(t *testing.T, body string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("AppData", dir) // os.UserConfigDir on Windows
	t.Setenv("HOME", dir)    // and on the other platforms
	if err := os.MkdirAll(filepath.Join(dir, "CKPresence"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "CKPresence", "config.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadConfigNullOrPartialSections(t *testing.T) {
	writeConfig(t, `{"situations":null}`) // used to panic: assignment to entry in nil map
	if got := LoadConfig(); len(got.Situations) != len(DefaultConfig().Situations) {
		t.Errorf("null situations did not fall back to the defaults: %+v", got.Situations)
	}
	writeConfig(t, `{"imageText":"Mine","showTimer":false}`)
	cfg := LoadConfig()
	if !cfg.Enabled || cfg.ShowTimer || cfg.UpdateSecs != 3 {
		t.Errorf("partial config lost its defaults: %+v", cfg)
	}
	if cfg.ImageText != "Mine" {
		t.Errorf("partial config was not read: %+v", cfg)
	}
	if cfg.Situations["dead"] != DefaultConfig().Situations["dead"] {
		t.Errorf("partial config lost the default situations: %+v", cfg.Situations)
	}
}

func TestLoadConfigKeepsEmptiedSituation(t *testing.T) {
	writeConfig(t, `{"situations":{"menu":{"details":"","state":""}}}`)
	cfg := LoadConfig() // used to put "In the main menu" back
	if cfg.Situations["menu"] != (Lines{}) {
		t.Errorf("emptied situation was resurrected: %+v", cfg.Situations["menu"])
	}
}

func TestLoadConfigUpgradesOnlyUntouchedDefaults(t *testing.T) {
	mine := Lines{Details: "mining", State: "{world}"}
	body, err := json.Marshal(map[string]any{"situations": map[string]Lines{
		"exploring": previousDefaults["exploring"], // never customised
		"boss":      mine,                          // written by the player
	}})
	if err != nil {
		t.Fatal(err)
	}
	writeConfig(t, string(body))
	cfg := LoadConfig()
	if cfg.Situations["exploring"] != DefaultConfig().Situations["exploring"] {
		t.Errorf("first-release default not upgraded: %+v", cfg.Situations["exploring"])
	}
	if cfg.Situations["boss"] != mine {
		t.Errorf("player wording overwritten: %+v", cfg.Situations["boss"])
	}
}

func TestCanSendBurstLimit(t *testing.T) {
	a := &App{cfg: Config{UpdateSecs: 3}}
	now := time.Now()
	sendAt := func(sec float64) bool {
		at := now.Add(time.Duration(sec * float64(time.Second)))
		if !a.canSend(at) {
			return false
		}
		a.sent = append(a.sent, at)
		return true
	}
	if !sendAt(0) || sendAt(2) || !sendAt(3) {
		t.Fatal("spacing: want a send at 0 s, none at 2 s, one at 3 s")
	}
	sendAt(6)
	sendAt(9)
	sendAt(12) // 5 sends within 20 s
	if sendAt(15) {
		t.Fatal("6th update inside 20 s should wait")
	}
	if !sendAt(20) { // the 0 s send has left the window
		t.Fatal("update should go out once the window frees up")
	}
}
