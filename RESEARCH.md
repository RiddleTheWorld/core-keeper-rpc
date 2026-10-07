# Core Keeper Rich Presence: Research Notes (2026-10-07)

Game: Steam app **1621690**, Unity + **Mono** (managed DLLs in `CoreKeeper_Data\Managed`), version 1.3.0.
Method: reflection-only loading of the game's assemblies (type and member names only, no code bodies),
then compiling `mod-check` against the real DLLs to confirm every signature below.

## Data the mod reads (verified)
| What | Where | Type |
|---|---|---|
| Local player | `Manager.main` → `Manager.player` | `PlayerController` |
| In a world (else main menu) | `Manager.sceneHandler.isInGame` | bool |
| World name | `Manager.saves.GetWorldName()` | string |
| Party size | `Manager.allPlayers` | `List<PlayerController>` |
| **Current biome** | `PlayerController.currentBiome` | enum `Biome` |
| In a safe zone (base) | `PlayerController.isInSafeZone` | bool |
| Character name | `PlayerController.playerName` | string |
| Position | `PlayerController.GetEntityPosition()` | `float3` (x/z are tiles) |
| Health | `PlayerController.currentHealth`, `GetMaxHealth()` | int |
| Food, mana | `PlayerController.hungerComponent.hunger`, `GetMana()`, `GetMaxMana()` | int |
| Held item | `PlayerController.GetHeldObject().objectID` | `ObjectID` |
| **Boss fight / special area** | `MusicManager.currentMusicRosterType` | enum `MusicRosterType` |
| Boss entity (nearest within 80 tiles) | ECS component `ObjectDataCD` + a fixed `ObjectID` set | struct |

### `Biome` enum
None=0, Slime=1, Larva=2, Stone=3, Obsidian=4, Nature=5, GreatWall=6, Sea=7, Desert=8, Crystal=9, Passage=10, Excavation=11

### `MusicRosterType` (the useful values)
- **Places:** HOME_BASE=11, MOLD_DUNGEON=17, CITY_DUNGEON=18, LAVA_DUNGEON=27, MEADOW=29, OASIS_BIOME=39, EVENT_TERMINAL=30, ROBOT_BOSS_DUNGEON=43, MOSS_BIOME=48
- **Bosses:** KING_SLIME=19, HIVE_MOTHER_FIGHT=21, GHORM=23, MALUGAZ=24, AZEOS=25, OMOROTH=26, SCARAB_BOSS=28, SNAKE_BOSS=32, CORE_COMMANDER_PHASE1/2=33/34, HYDRA_BOSS=36, GIANT_CICADA_BOSS=38, ROBOT_BOSS_PHASE1/2=40/41, GIANT_DRILL=44, IVY_THE_POISONOUS_MASS=45, IGNEOUS_THE_MOLTEN_MASS=46, MORPHA_THE_AQUATIC_MASS=47
- **Menus:** TITLE=1, INTRO=2, CREDITS=6, OUTRO=35

## Mod system (verified)
- Mods are **C# source compiled by the game at load time** (the game ships Roslyn and `mcs.dll`), loaded from `CoreKeeper_Data\StreamingAssets\Mods\<name>\` - where `DiscordPresence` is installed.
- **Harmony** (`0Harmony.dll`) is available for patching game methods; `DisableBuiltInPresence` uses it.
- `PugMod.ModMetadata`: guid, name, displayName, skipSafetyChecks, disableScripts, accessesExtraAssemblies, disableHarmonyPatching, requiredOn, files, dependencies.
- `PugMod.InvokeChecker` keeps a `_disallowedTypes` list. **Mods are sandboxed**: the mod writes no files, opens no sockets and uses no reflection. It reports through `Debug.Log`, which the game already writes to `Player.log`.
- `PugMod.API` (PugMod.SDK.Runtime) + `ModAPIConfig` (config-file API). `IMod` is the entry point: `EarlyInit`, `Init`, `Update`, `ModObjectLoaded`, `Shutdown`, `CanBeUnloaded`.

## Discord side
- Rich Presence needs the Discord **desktop** app's IPC pipe (`\\.\pipe\discord-ipc-N`).
- The app reuses Core Keeper's own Discord application id (`clientID` in `app\app.go`), so the status shows the game's card.
- Tools that strip Discord's `discord_rpc` module remove the pipe, and the status stops showing.

## What shipped
1. **Mod (C#):** every second reads the fields above; on a change (or a 30 s heartbeat) it writes one `[CKRP]|v1|...` line to `Player.log` (format in `DiscordPresenceMod.cs`), and patches the game's own Discord status off.
2. **App (Go + Wails):** tails `Player.log`, turns the newest line into a situation, fills in the player's wording (`config.json`) and sends `SET_ACTIVITY` over the pipe itself.

The earlier plan - a `status.json` written by the mod and watched by a small Rust companion - was dropped: a log line needs no file writes from inside the sandbox and no second process.
