// A stand-in for the reference's Settings, with only what the matcher reads.
using Flow.Launcher.Plugin.SharedModels;

namespace Flow.Launcher.Infrastructure.UserSettings
{
    public class Settings
    {
        public SearchPrecisionScore QuerySearchPrecision { get; set; } = SearchPrecisionScore.Regular;
        public bool IgnoreAccents { get; set; } = false;
    }
}
