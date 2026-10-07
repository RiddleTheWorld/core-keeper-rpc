// Turns off Core Keeper's own Discord status, which Discord would show instead of ours.
// Trade-off: the game's Discord "Ask to Join" invites don't work while this mod is installed.
using HarmonyLib;
using PugPlatform;

[HarmonyPatch(typeof(DiscordPlatform), nameof(DiscordPlatform.Init))]
public static class SkipBuiltInDiscordInit
{
    public static bool Prefix()
    {
        return false;
    }
}

[HarmonyPatch(typeof(DiscordPlatform), nameof(DiscordPlatform.Update))]
public static class StopBuiltInDiscordUpdates
{
    private static bool _stopped;

    public static bool Prefix(DiscordPlatform __instance)
    {
        if (!_stopped)
        {
            _stopped = true;
            try
            {
                __instance.Deinit(); // may throw if Init never ran; that's fine
            }
            catch (System.Exception)
            {
            }
        }
        return false;
    }
}
