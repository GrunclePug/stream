# stream

A dwm-inspired tiling window manager for river.

![stream with its h2o theme: waybar showing the tags, layout and status over the h2o wallpaper](docs/screenshot.png)

## Overview

stream is a window manager for the [river](https://codeberg.org/river/river) Wayland compositor. river does no window management itself and leaves all policy to one client speaking its `river-window-management-v1` protocol; `swm` is that client. stream follows dwm's model (per-monitor tags, master and stack tiling) and ships as two static Go binaries: `swm`, the window manager, and `streamctl`, which controls it.

## Features

* **dwm tiling**: master and stack with gaps, monocle and floating layouts. New windows become the master.
* **dwm tags**: nine per monitor. A window can be on several, several can be shown at once, and each tag keeps its own layout.
* **Several monitors**: each with its own tags. Windows sent across land on the shown tags, and nothing is lost when a monitor is unplugged or you switch VTs.
* **Mouse**: focus follows the pointer. Mod+drag moves and resizes floating windows, moves tiled ones around the layout, and drags the dividers between them.
* **Floating where it fits**: dialogs and fixed-size windows float, and fullscreen requests are honoured. Windows show only a border.
* **Scratchpads and rules**: windows one key shows and hides, and rules that float, tag or place windows by app-id and title.
* **One plain config file**: dwm's keys are built in, and `streamctl gen-config` writes them out to edit. Binding modes, keys that run several commands, and mistakes reported by line and never applied.
* **Scriptable**: `streamctl` runs any command in the running `swm`, and `streamctl watch` streams its state as JSON for waybar or any other bar.
* **Restart-safe**: river keeps every window when `swm` exits, and `swm` puts each one back on its monitor, tags and place.
* **h2o**: stream's own look and wallpaper, with the full theme for foot, mako, tmux, dunst, GTK and waybar as an example.
* **Built for river**: uses river's manage and render sequences, so every change appears in a single frame. Pure Go, no cgo.

## Installation

### Requirements

* Go 1.27.3 or newer
* Make
* libxkbcommon headers (`xkbcommon-keysyms.h`), at build time only
* river 0.5.0 or newer
* swaybg, or another wallpaper program, for the wallpaper (optional)
* waybar and jq, for the bar setup in [Bars](docs/bars.md) (optional)

### Build

```sh
git clone https://github.com/grunclepug/stream.git
cd stream
make build
sudo make install
```

## Usage

river runs `~/.config/river/init` on startup. Start `swm` from there, restarting it whenever it exits, and then any other long-running programs:

```sh
#!/bin/sh
while :; do
	swm
	sleep 1
done &

streamctl wait
waybar &
swaybg -i /usr/local/share/backgrounds/stream/h2o.png -m fill &
```

The loop restarts `swm` whenever it exits; river keeps every window meanwhile, and `swm` puts them back. `streamctl wait` holds the bar and wallpaper back until `swm` is running, since river closes them otherwise. [Getting started](docs/getting-started.md) explains each line.

Then start river from a TTY:

```sh
river
```

### Default bindings

Mod is Alt. These are dwm's keys, with stream's extras on Super.

| Key | Action |
|-----|--------|
| Mod+p | Launcher (`wmenu-run`) |
| Mod+Shift+Return | Terminal (`foot`) |
| Super+Return | Scratchpad terminal |
| Mod+j / Mod+k | Focus next / previous window |
| Mod+Shift+j / Mod+Shift+k | Swap with next / previous window |
| Mod+Return | Zoom: make the focused window the master |
| Mod+Shift+c | Close the focused window |
| Mod+Shift+space | Float or tile the focused window |
| Mod+Shift+f | Fullscreen |
| Mod+t / Mod+f / Mod+m | Tile / floating / monocle layout |
| Mod+space | Previous layout |
| Mod+h / Mod+l | Shrink / grow the master area |
| Mod+i / Mod+d | More / fewer master windows, up to the windows shown |
| Mod+minus / Mod+equal | Smaller / bigger gaps |
| Mod+Shift+equal | Configured gap |
| Mod+1..9 | View a tag |
| Mod+Ctrl+1..9 | Toggle a tag in the view |
| Mod+Shift+1..9 | Move the focused window to a tag |
| Mod+Ctrl+Shift+1..9 | Toggle a tag on the focused window |
| Mod+0 / Mod+Shift+0 | View all tags / put the window on all tags, or back on the shown ones |
| Mod+Tab | Previous tags |
| Mod+comma / Mod+period | Focus previous / next monitor |
| Mod+Shift+comma / Mod+Shift+period | Send the window to the previous / next monitor |
| Mod+Shift+r | Reload the config |
| Mod+Shift+q | Quit river |
| Super+F11 | Passthrough mode, for nested sessions (Super+F11 again to leave) |
| Mod+left drag | Move a floating window, or reorder a tiled one |
| Mod+middle click | Float or tile the window |
| Mod+right drag | Resize a floating window, or move a divider |

### Configuration

`swm` runs on built-in defaults until you give it a config file. To get one to edit, write the defaults out:

```sh
streamctl gen-config        # writes ~/.config/stream/config
streamctl gen-config -o     # prints it instead
```

Edit it, then reload with Mod+Shift+r. A file with mistakes changes nothing: `swm` keeps the config it has and reports each problem with its line number, and `streamctl gen-config -check` checks a file without loading it. [Configuration](docs/configuration.md) covers everything the file can hold.

| Flag | Default | Description |
|------|---------|-------------|
| `-config` | `$XDG_CONFIG_HOME/stream/config` | Config file to read. Falls back to `~/.config/stream/config`. |
| `-log-level` | `info` | Log level: `debug`, `info`, `warn` or `error`. Logs go to stderr. |
| `-version` | | Print the version and exit. |

## Documentation

The [documentation](docs/README.md) covers everything in depth:

* [Getting started](docs/getting-started.md): from installing to a working session.
* [Configuration](docs/configuration.md): settings, bindings, modes, rules, scratchpads and every command.
* [Tiling](docs/tiling.md), [Windows](docs/windows.md) and [Monitors](docs/monitors.md): how stream arranges, focuses and moves windows.
* [Bars](docs/bars.md): showing stream's state in waybar or any other bar.
* [Theming](docs/theming.md): using the h2o theme, and making your own.
* [streamctl](docs/streamctl.md), [How it works](docs/how-it-works.md) and [Troubleshooting](docs/troubleshooting.md).

## h2o

h2o is stream's theme: clear water blue on deep water. Its border colours are stream's defaults, and `make install` installs its wallpaper as `$(PREFIX)/share/backgrounds/stream/h2o.png`.

stream itself only draws borders. The rest of the theme, for foot, mako, tmux, dunst, GTK and waybar, is in [examples/h2o](examples/h2o). [Theming](docs/theming.md) shows how to use it, and how to make your own theme from it.

## Development

* `make run`: build a development `swm` and run it in river, nested in a window when started from a Wayland session.
* `make test`: run the tests.
* `make lint`: run `go vet` and staticcheck.
* `make build`: build `bin/swm` and `bin/streamctl`.
* `make clean`: remove build output and generated code.

## Contributing

Fork the repository and open a pull request.

## License

MIT. See [LICENSE](LICENSE).

## Author

Chad Humphries | [Website](https://grunclepug.com/) | [GitHub](https://github.com/GrunclePug) | [More projects](https://github.com/GrunclePug?tab=repositories)
