# HyprTracker

An app usage tracker for [Hyprland](https://github.com/hyprwm/Hyprland).

```sh
$ hyprtracker -help
Usage of hyprtracker:
  -app-only
        Only display per-application report, skip window details
  -daemon
        Run as a daemon to collect window activity
  -db-path string
        Path to the SQLite database file
  -general-debounce int
        General debounce time in seconds (default: 0.5)
  -idle-signal string
        Send idle signal to running daemon: 'start' to mark idle start, 'end' to mark idle end
  -keywords string
        Comma-separated list of keywords to filter related activities (e.g., "firefox,projectX,mydoc")
  -min-duration int
        Minimum duration in seconds to include in the output (e.g., 1 will filter out activities less than 1 second) (default 60)
  -systray
        Enable system tray icon for controlling the daemon (default true)
  -terminal-debounce int
        Terminal debounce time in seconds (default 3)
  -time-range string
        Time range for analysis: 'day', 'week', 'month', 'year', or 'all' (default "month")
  -toggle-pause
        Toggle pause/resume on a running daemon       Toggle pause/resume on a running daemon
```

## Idle Manager Integration

You can use idle managers like `hypridle` to avoid tracking inactive periods:

```
# hypridle configuration
listener {
    timeout = 300  # 5 minutes
    on-timeout = hyprtracker -idle-signal start
    on-resume = hyprtracker -idle-signal end
}
```