# Core Keeper RPC

Shows what you're doing in Core Keeper on your Discord profile, in your own words:

```
Playing Core Keeper
Fighting Malugaz the Corrupted
❤ 245/300 · Iron Sword · Solo
```

It has two parts: a **mod** that reads the game, and an **app** that sends it to Discord.

## Install

Download the files from the [latest release](../../releases/latest).

**1. The mod**
1. Unzip `DiscordPresence-mod.zip`.
2. Put the `DiscordPresence` folder in `Core Keeper\CoreKeeper_Data\StreamingAssets\Mods\`.
   To find it in Steam: right-click Core Keeper → Manage → Browse local files.

**2. The app**
- `CKPresence.exe`: just run it, no install needed.
- Or `CKPresence-installer.exe` if you want a Start menu shortcut.

Needs Windows 10/11 and the Discord desktop app.

## Use

1. Open the app, then start Core Keeper. Keep Discord running.
2. Pick a situation on the left (Exploring, Boss fight, ...) and write what Discord should say.
3. Click a token like `{biome}` or `{hp}` to insert live game info. Parts split by `·` hide themselves when their info is empty.

Closing the window keeps it running in the background. Use **Options → Quit app** to stop it.

**Tokens:** `{biome}` `{place}` `{boss}` `{hp}` `{maxhp}` `{food}` `{mana}` `{maxmana}` `{held}` `{distance}` `{x}` `{y}` `{partytext}` `{party}` `{world}` `{character}`

`{place}` only fills in special areas (dungeons, your base) and `{boss}` only during a boss fight.

**Good to know**
- The mod turns off the game's own Discord status so yours shows instead. The game's "Ask to Join" invites won't work while it's installed.
- Discord accepts at most 5 updates in 20 seconds, so after a burst of changes your status can lag a few seconds.

## Uninstall

Delete the `DiscordPresence` mod folder and the app (or uninstall it from Windows Settings → Apps). Settings are in `%APPDATA%\CKPresence`.

## Building from source

Needs Go, Node and the [Wails v2 CLI](https://wails.io), plus NSIS for the installer.
- App: `cd app`, then `wails build` (add `-nsis` for the installer). Tests: `go test .`
- Mod compile check, with the game installed: `cd mod-check`, then `dotnet build`. For a non-default game folder, add `-p:GameManaged="<game>\CoreKeeper_Data\Managed"`.

How the game was analysed: [RESEARCH.md](RESEARCH.md).
