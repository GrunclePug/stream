# Troubleshooting

## Getting logs

`swm` logs to its standard error, which river passes on to its own. To keep a session's logs in a file, start river with its output redirected:

```sh
river >~/river.log 2>&1
```

For more detail, run `swm` with debug logging in the river init. It logs every binding pressed, command run and focus change:

```sh
while :; do
	swm -log-level debug
	sleep 1
done &
```

## The bar or wallpaper doesn't appear

river closes bars, wallpapers and other layer-shell surfaces that start before a window manager is connected, and most programs don't try again. Start them after `streamctl wait` in the river init:

```sh
streamctl wait
swaybg -i /usr/local/share/backgrounds/stream/h2o.png -m fill &
waybar &
```

Once they're up, they stay up across `swm` restarts.

## Keys do nothing

* **Is `swm` running?** `streamctl version` works without it; `streamctl view 1` doesn't. If the river init's loop keeps restarting `swm`, its log says why.
* **Are you in another mode?** In the default passthrough mode, Super+F11 is the only binding: press it to get back. A bar can show the active mode: see [Bars](bars.md).
* **Is Mod the key you think?** Mod is Alt unless your config sets `mod`.
* **Does the config load?** `streamctl gen-config -check` reports its problems. While the config has errors, `swm` keeps the last one that loaded, or its defaults when starting.
* **Is the screen locked?** Only `-locked` bindings work then.

## Config changes don't apply

`swm` reads the config when it starts and on `reload` (Mod+Shift+r), not when the file changes. Run `streamctl reload` to see any errors or warnings. If your config has bindings, a binding you deleted is gone even though it's a default: see [Configuration](configuration.md#the-file).

## A program opens in the wrong place

Rules match a window's app-id and title when it opens. To see what a window reports, focus it and run `streamctl watch`. Some programs set their title only after opening, so a rule on the title can miss them; match the app-id instead.

## Windows have title bars

stream asks every program to leave out its title bar. Programs that insist draw their own, usually GTK apps with header bars. There's nothing to change in stream for these.

## streamctl says swm isn't running

`streamctl` finds `swm` through `$XDG_RUNTIME_DIR` and `$WAYLAND_DISPLAY`. Inside the session they're set; from a TTY or over ssh, set them to the session's values first, such as `WAYLAND_DISPLAY=wayland-1`.
