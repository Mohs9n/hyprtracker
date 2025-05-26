# HyprTracker

An app usage tracker for [Hyprland](https://github.com/hyprwm/Hyprland).

## Usage
```bash
$ hyprtracker -help                      
Usage of hyprtracker:
  -apps
        Only display per-application report, skip window details
  -daemon
        Run as a daemon to collect window activity
  -keywords string
        Comma-separated list of keywords to filter related activities (e.g., "firefox,projectX,mydoc")
  -logfile string
        Path to the log file (default "/home/mohsen/.local/share/hyprtracker/hyprland_activity.log")
  -min int
        Minimum duration in seconds to include in the output (e.g., 1 will filter out activities less than 1 second)
```