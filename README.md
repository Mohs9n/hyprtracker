# HyprTracker

An app usage tracker for [Hyprland](https://github.com/hyprwm/Hyprland).

## Usage
```bash
hyprtracker -daemon #start logging activity

hyprtracker # prints a breakdown of the logged activity
hyprtracker -keywords "firefox,projectX,mydoc" # filter breakdown results with some comma seperated keywords"
hyprtracker -min-duration 1 # filter out activities that lasted less than 1 second
hyprtracker -keywords "code" -min-duration 60 # combine filters (only show coding activities that lasted at least a minute)
```