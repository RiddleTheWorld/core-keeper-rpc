// Discord Rich Presence for Core Keeper - in-game half.
//
// Every second this reads what the local player is doing (biome, boss fight, health, food,
// mana, held item, position, party, world) and, when it changes (or every 30 s as a heartbeat),
// writes ONE line to the game's Player.log:
//
//   [CKRP]|v1|game|<world>|<character>|<biome>|<place>|<boss>|<hp>|<maxHp>|<party>|<safe>|<roster>|<food>|<mana>|<maxMana>|<held>|<x>|<y>
//   [CKRP]|v1|menu
//
// The companion app (CKPresence.exe) reads those lines and updates Discord.
// No file, network or reflection APIs are used, so the mod passes the game's safety checks.
using System;
using System.Collections.Generic;
using PugMod;
using Unity.Collections;
using Unity.Entities;
using Unity.Mathematics;
using Unity.Transforms;
using UnityEngine;

public class DiscordPresenceMod : IMod
{
    private const float PollSeconds = 1f;
    private const float HeartbeatSeconds = 30f;
    private const float BossSearchRadius = 80f;
    private const string Prefix = "[CKRP]";

    private float _nextPoll;
    private float _lastEmit = -999f;
    private string _lastLine;

    public void EarlyInit()
    {
        Debug.Log(Prefix + "|v1|loaded");
    }

    public void Init()
    {
        // Required by IMod; this mod has nothing to set up.
    }

    public void Shutdown()
    {
        Debug.Log(Prefix + "|v1|quit");
    }

    public void ModObjectLoaded(UnityEngine.Object obj)
    {
        // Required by IMod; this mod is not interested in any mod objects.
    }

    public bool CanBeUnloaded()
    {
        return true;
    }

    public void Update()
    {
        float now = Time.realtimeSinceStartup;
        if (now < _nextPoll)
        {
            return;
        }
        _nextPoll = now + PollSeconds;

        string line;
        try
        {
            line = BuildStatusLine();
        }
        catch (Exception)
        {
            // No exception type name here: GetType() is reflection, which the game's mod sandbox rejects.
            line = Prefix + "|v1|error";
        }

        if (line != _lastLine || now - _lastEmit >= HeartbeatSeconds)
        {
            Debug.Log(line);
            _lastLine = line;
            _lastEmit = now;
        }
    }

    private static string BuildStatusLine()
    {
        Manager main = Manager.main;
        SceneHandler scene = Manager.sceneHandler;
        PlayerController player = main != null ? main.player : null;
        if (scene == null || !scene.isInGame || player == null)
        {
            return Prefix + "|v1|menu";
        }

        MusicRosterType roster = Manager.music != null ? Manager.music.currentMusicRosterType : MusicRosterType.DEFAULT;
        string boss = BossRosters.ContainsKey(roster) ? FindBossName(player, roster) : "";
        int party = main.allPlayers != null ? Math.Max(1, main.allPlayers.Count) : 1;
        Vector3 pos = player.GetEntityPosition();

        return string.Join("|", new string[]
        {
            Prefix,
            "v1",
            "game",
            Clean(WorldName()),
            Clean(player.playerName),
            BiomeName(player.currentBiome),
            PlaceName(roster),
            Clean(boss),
            player.currentHealth.ToString(),
            player.GetMaxHealth().ToString(),
            party.ToString(),
            player.isInSafeZone ? "1" : "0",
            roster.ToString(),
            SafeInt(() => player.hungerComponent.hunger),
            SafeInt(() => player.GetMana()),
            SafeInt(() => player.GetMaxMana()),
            Clean(HeldItemName(player)),
            ((int)Math.Round(pos.x)).ToString(),
            ((int)Math.Round(pos.z)).ToString(),
        });
    }

    private static string SafeInt(Func<int> read)
    {
        try
        {
            return read().ToString();
        }
        catch (Exception)
        {
            return "";
        }
    }

    // The item in the player's hand, by its in-game name ("" when empty-handed).
    private static string HeldItemName(PlayerController player)
    {
        try
        {
            ObjectID id = player.GetHeldObject().objectID;
            return id == ObjectID.None ? "" : Localized("Items/" + id, Spaced(id.ToString()));
        }
        catch (Exception)
        {
            return "";
        }
    }

    private static string WorldName()
    {
        try
        {
            SaveManager saves = Manager.saves;
            return saves != null ? saves.GetWorldName() : "";
        }
        catch (Exception)
        {
            return "";
        }
    }

    // --- bosses

    private static readonly HashSet<ObjectID> BossIds = new HashSet<ObjectID>
    {
        ObjectID.SlimeBoss, ObjectID.PoisonSlimeBoss, ObjectID.SlipperySlimeBoss, ObjectID.LavaSlimeBoss,
        ObjectID.BossLarva, ObjectID.LarvaHiveBoss, ObjectID.LarvaHiveHalloweenBoss, ObjectID.ShamanBoss,
        ObjectID.BirdBoss, ObjectID.OctopusBoss, ObjectID.ScarabBoss,
        ObjectID.HydraBossNature, ObjectID.HydraBossSea, ObjectID.HydraBossDesert, ObjectID.HydraBossVoid,
        ObjectID.CoreBoss, ObjectID.WallBoss, ObjectID.GiantCicadaBoss, ObjectID.RobotBoss, ObjectID.GiantDrill,
    };

    // Boss music roster to the boss entity it stands for; ObjectID.None where the roster only
    // says "a boss fight", so the entity is found by proximity instead.
    private static readonly Dictionary<MusicRosterType, ObjectID> BossRosters = new Dictionary<MusicRosterType, ObjectID>
    {
        { MusicRosterType.BOSS, ObjectID.None },
        { MusicRosterType.KING_SLIME, ObjectID.SlimeBoss },
        { MusicRosterType.HIVE_MOTHER_FIGHT, ObjectID.LarvaHiveBoss },
        { MusicRosterType.GHORM, ObjectID.BossLarva },
        { MusicRosterType.MALUGAZ, ObjectID.ShamanBoss },
        { MusicRosterType.AZEOS, ObjectID.BirdBoss },
        { MusicRosterType.OMOROTH, ObjectID.OctopusBoss },
        { MusicRosterType.SCARAB_BOSS, ObjectID.ScarabBoss },
        { MusicRosterType.SNAKE_BOSS, ObjectID.None },
        { MusicRosterType.CORE_COMMANDER_PHASE1, ObjectID.CoreBoss },
        { MusicRosterType.CORE_COMMANDER_PHASE2, ObjectID.CoreBoss },
        { MusicRosterType.HYDRA_BOSS, ObjectID.None },
        { MusicRosterType.GIANT_CICADA_BOSS, ObjectID.GiantCicadaBoss },
        { MusicRosterType.ROBOT_BOSS_PHASE1, ObjectID.RobotBoss },
        { MusicRosterType.ROBOT_BOSS_PHASE2, ObjectID.RobotBoss },
        { MusicRosterType.GIANT_DRILL, ObjectID.GiantDrill },
        { MusicRosterType.IVY_THE_POISONOUS_MASS, ObjectID.PoisonSlimeBoss },
        { MusicRosterType.IGNEOUS_THE_MOLTEN_MASS, ObjectID.LavaSlimeBoss },
        { MusicRosterType.MORPHA_THE_AQUATIC_MASS, ObjectID.SlipperySlimeBoss },
    };

    // The nearest boss entity to the player (within BossSearchRadius), else the boss the
    // music implies. Only runs while boss music is playing.
    private static string FindBossName(PlayerController player, MusicRosterType roster)
    {
        ObjectID found = ObjectID.None;
        try
        {
            World world = player.world;
            if (world != null && world.IsCreated)
            {
                EntityManager em = world.EntityManager;
                EntityQuery query = em.CreateEntityQuery(ComponentType.ReadOnly<ObjectDataCD>(), ComponentType.ReadOnly<LocalTransform>());
                NativeArray<ObjectDataCD> data = query.ToComponentDataArray<ObjectDataCD>(Allocator.Temp);
                NativeArray<LocalTransform> transforms = query.ToComponentDataArray<LocalTransform>(Allocator.Temp);
                float3 playerPos = player.GetEntityPosition();
                float best = BossSearchRadius * BossSearchRadius;
                for (int i = 0; i < data.Length; i++)
                {
                    if (!BossIds.Contains(data[i].objectID))
                    {
                        continue;
                    }
                    float d = math.distancesq(transforms[i].Position, playerPos);
                    if (d < best)
                    {
                        best = d;
                        found = data[i].objectID;
                    }
                }
                data.Dispose();
                transforms.Dispose();
                query.Dispose();
            }
        }
        catch (Exception)
        {
            found = ObjectID.None;
        }

        if (found == ObjectID.None && BossRosters.TryGetValue(roster, out ObjectID byMusic))
        {
            found = byMusic;
        }
        if (found == ObjectID.None)
        {
            return "a boss";
        }
        return Localized("Items/" + found, Spaced(found.ToString()));
    }

    // --- names

    private static string BiomeName(Biome b)
    {
        switch (b)
        {
            case Biome.Slime: return "Dirt Biome";
            case Biome.Larva: return "Clay Caves";
            case Biome.Stone: return "Forgotten Ruins";
            case Biome.Nature: return "Azeos' Wilderness";
            case Biome.Sea: return "Sunken Sea";
            case Biome.Desert: return "Desert of Beginnings";
            case Biome.Crystal: return "Shimmering Frontier";
            case Biome.Passage: return "The Passage";
            case Biome.Excavation: return "Excavation Site";
            default: return "";
        }
    }

    private static string PlaceName(MusicRosterType r)
    {
        switch (r)
        {
            case MusicRosterType.HOME_BASE: return "Home Base";
            case MusicRosterType.MOLD_DUNGEON: return "Mold Dungeon";
            case MusicRosterType.CITY_DUNGEON: return "Ancient City";
            case MusicRosterType.LAVA_DUNGEON: return "Lava Dungeon";
            case MusicRosterType.MEADOW: return "Meadow";
            case MusicRosterType.OASIS_BIOME: return "Oasis";
            case MusicRosterType.HIVE_MOTHER_SLEEPING: return "Hive Mother's Lair";
            default: return "";
        }
    }

    private static string Localized(string term, string fallback)
    {
        try
        {
            string s = API.Localization.GetLocalizedTerm(term);
            if (!string.IsNullOrEmpty(s) && s != term)
            {
                return s;
            }
        }
        catch (Exception)
        {
        }
        return fallback;
    }

    // "LavaSlimeBoss" -> "Lava Slime Boss" (fallback when no translation exists).
    private static string Spaced(string id)
    {
        var sb = new System.Text.StringBuilder(id.Length + 8);
        for (int i = 0; i < id.Length; i++)
        {
            if (i > 0 && char.IsUpper(id[i]) && !char.IsUpper(id[i - 1]))
            {
                sb.Append(' ');
            }
            sb.Append(id[i]);
        }
        return sb.ToString();
    }

    // Keeps the log line parseable: no separators or line breaks, max 64 chars.
    private static string Clean(string s)
    {
        if (string.IsNullOrEmpty(s))
        {
            return "";
        }
        s = s.Replace('|', '/').Replace('\r', ' ').Replace('\n', ' ').Trim();
        return s.Length > 64 ? s.Substring(0, 64) : s;
    }
}
