# Getting started

## What you need

* river 0.5.0 or newer.
* Go, Make and the libxkbcommon headers, to build.
* For the default bindings: [foot](https://codeberg.org/dnkl/foot) as the terminal and [wmenu](https://codeberg.org/adnano/wmenu) as the launcher. You can change both later.
* Optional: swaybg for the wallpaper, and a bar, such as waybar: see [Bars](bars.md).

## Install

```sh
git clone https://github.com/grunclepug/stream.git
cd stream
make build
sudo make install
```

This installs `swm`, the window manager, and `streamctl`, which controls it, to `/usr/local/bin`, and stream's wallpaper to `/usr/local/share/backgrounds/stream/h2o.png`. Set `PREFIX` to install elsewhere, such as `sudo make install PREFIX=/usr`.

## The river init

river starts with nothing on screen. On startup it runs `~/.config/river/init`, and that script starts the window manager and everything else. Create it and make it executable:

```sh
#!/bin/sh
while :; do
	swm
	sleep 1
done &

streamctl wait
swaybg -i /usr/local/share/backgrounds/stream/h2o.png -m fill &
waybar &
```

```sh
chmod +x ~/.config/river/init
```

What each part does:

* **The loop** runs `swm` and starts it again whenever it exits. river keeps every window open while `swm` is gone, and the new `swm` puts them back where they were, so a crash costs about a second. It also means trying a new build is just `pkill -x swm`. See [How it works](how-it-works.md#restarts).
* **`streamctl wait`** returns once `swm` is running. river closes bars and wallpapers that start before a window manager is connected, so start them after this line. Once they're up, they stay up across `swm` restarts.
* **swaybg and waybar** are examples. Start anything long-running here, such as a notification daemon.

To end the session, press Mod+Shift+q. river stops, and everything the init started stops with it.

## First session

Start river from a TTY:

```sh
river
```

With a display manager, such as greetd or SDDM, pick the River session instead; it runs the same init.

Mod is Alt. Some keys to begin with:

| Key | Action |
|-----|--------|
| Mod+Shift+Return | Open a terminal |
| Mod+p | Open the launcher |
| Mod+j / Mod+k | Focus the next / previous window |
| Mod+Return | Make the focused window the master |
| Mod+1..9 | View a tag |
| Mod+Shift+1..9 | Move the focused window to a tag |
| Mod+Shift+c | Close the focused window |
| Mod+Shift+q | Quit |

The README lists [every default binding](../README.md#default-bindings).

## Your config

`swm` runs on built-in defaults until you give it a config file. Write the defaults out as a file to edit:

```sh
streamctl gen-config
```

This writes `~/.config/stream/config`, with every default setting and binding and comments explaining the format. Edit it, then press Mod+Shift+r to reload. If the file has a mistake, `swm` keeps the config it has and reports each problem with its line number. To check a file without reloading:

```sh
streamctl gen-config -check
```

[Configuration](configuration.md) explains everything the file can hold.

## Keeping up with new defaults

Once your config has its own bindings, they replace all of stream's default bindings, so bindings added in a newer version don't appear on their own. After updating stream, run:

```sh
streamctl gen-config -missing
```

It prints the default bindings your config doesn't have, as lines to paste into it.

## Next

* [Tiling](tiling.md) and [Windows](windows.md) explain how stream arranges and focuses windows.
* [Bars](bars.md) sets up waybar to show your tags.
* [Theming](theming.md) applies the h2o theme to your terminal, notifications and bar.
