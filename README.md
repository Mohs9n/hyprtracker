# HyprTracker

An app usage tracker for [Hyprland](https://github.com/hyprwm/Hyprland).

## Usage
```bash
$ hyprtracker -help                     
Usage of ./hyprtracker:
  -app-only
        Only display per-application report, skip window details
  -daemon
        Run as a daemon to collect window activity
  -general-debounce int
        General debounce time in seconds (default: 0.5)
  -idle-signal string
        Send idle signal to running daemon: 'start' to mark idle start, 'end' to mark idle end
  -keywords string
        Comma-separated list of keywords to filter related activities (e.g., "firefox,projectX,mydoc")
  -logfile string
        Path to the log file
  -min-duration int
        Minimum duration in seconds to include in the output (e.g., 1 will filter out activities less than 1 second)
  -terminal-debounce int
        Terminal debounce time in seconds (default: 3) (default 3)
  -use-external-idle
        Use external idle manager (default true)
```

## Idle Manager Integration
```bash
hyprtracker -daemon

# In your hypridle configuration, add these commands to signal idle state:
listener {
    timeout = 300  # 5 minutes
    on-timeout = hyprtracker -idle-signal start
    on-resume = hyprtracker -idle-signal end
}
```