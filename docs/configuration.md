# Configuration

`swm` reads `$XDG_CONFIG_HOME/stream/config`, or `~/.config/stream/config` when that variable isn't set. Without the file, it runs on its built-in defaults. `streamctl gen-config` writes those defaults out as a commented file to start from; the `-config` flag points `swm` at another file.

## The file

One directive per line. Lines starting with `#` are comments. A `#` anywhere else is part of the line, as in a colour, so comments can't follow a directive on the same line.

Any setting you leave out keeps its default. Bindings, rules, scratchpads and input lines work differently: once the file has any `bind` line, its bindings replace all the default bindings, and the same goes for `rule`, `scratchpad` and `input` lines. That way the file always shows every binding you have.

## Settings

| Setting | Default | Description |
|---------|---------|-------------|
| `border-width` | `1` | Border width in pixels. |
| `border-focused` | `#3fa9f5` | Border colour of the focused window, `#rrggbb` or `#rrggbbaa`. |
| `border-unfocused` | `#1f3a52` | Border colour of other windows. |
| `layout` | `tile` | Layout every tag starts with: `tile`, `monocle` or `floating`. |
| `nmaster` | `1` | Windows in the master area. |
| `mfact` | `0.55` | Share of the width the master area takes, `0.05` to `0.95`. |
| `gap` | `5` | Gap in pixels between windows and at the screen edges. |
| `mod` | `alt` | The modifier `Mod` stands for in bindings: `alt`, `super`, `ctrl`, `shift`, `mod3` or `mod5`. |
| `repeat-delay` | `600` | Milliseconds a key is held before it repeats, in programs and for `-repeat` bindings. |
| `repeat-rate` | `25` | Repeats per second after that. |

## Includes

```
include [-optional] <path>
```

Reads another file in place, as if its lines were here. Paths can start with `~/` or be relative to the including file. With `-optional`, a missing file is skipped; problems inside a file that exists still count. Themes use this to keep colours in their own file: see [Theming](theming.md).

## Bindings

```
bind [-repeat] [-locked] [-also] <keys> <command>
```

Keys are modifiers and a key joined by `+`, such as `Mod+Shift+Return`.

* **Modifiers** are `Mod`, `Super`, `Alt`, `Ctrl` and `Shift`. `Mod` stands for the `mod` setting, so changing `mod` moves every `Mod` binding at once.
* **Keys** are xkb keysym names: `Return`, `space`, `comma`, `XF86AudioMute`. Names ignore case. Write letters in lower case, adding `Shift` for capitals: `Mod+Shift+c`, not `Mod+C`. The same goes for other keys: bind `Mod+Shift+1`, not the `exclam` that Shift turns 1 into.
* **Pointer buttons** are `BTN_LEFT`, `BTN_MIDDLE` and `BTN_RIGHT`, as in `Mod+BTN_LEFT`. A pointer binding first focuses the window under the pointer, then runs its command on it.

Flags:

* `-repeat` runs the command again while the key is held, after `repeat-delay` and at `repeat-rate`.
* `-locked` keeps the binding working while the screen is locked, such as volume keys. All other bindings are off while it's locked.
* `-also` marks a binding that deliberately shares its key with an earlier one (see below).

### One key, several commands

Bind a key more than once and a press runs each command, in the order the file lists them:

```
bind Mod+r reload
bind -also Mod+r spawn notify-send "config reloaded"
```

`Mod` and the modifier it stands for are the same key: with `mod alt`, `Mod+q` and `Alt+q` are one key. Because a key bound twice is often a mistake, `swm` warns about it unless the later bindings say `-also`.

### Modes

Top-level bindings belong to the `normal` mode, which `swm` starts in. Other modes are blocks of bindings, and the `mode` command switches between them:

```
bind Super+F11 mode passthrough

mode passthrough {
	bind Super+F11 mode normal
}
```

Only the active mode's bindings work, so a mode can give keys new meanings, or almost none, as the passthrough mode above does for nested sessions and virtual machines. Every mode needs a way back to `normal`, so a config can't leave the keyboard stuck. Bars can show the active mode: see [Bars](bars.md).

## Rules

```
rule [-app-id <glob>] [-title <glob>] <action>...
```

A rule sets up each new window whose app-id and title match. In the patterns, `*` matches any text and `?` any one character, and double quotes keep spaces in a value. Actions:

| Action | Effect |
|--------|--------|
| `float` | Open floating. |
| `tile` | Open tiled, even if it would float by itself, as dialogs do. |
| `tag <n>` | Open on tag n. Repeat it for several tags. |
| `monitor <name>` | Open on that monitor, named by its output, such as `DP-1` or `eDP-1`. |

Every matching rule applies, in order:

```
rule -app-id gimp* float
rule -title "Picture-in-Picture" float
rule -app-id firefox monitor DP-1
```

A window's app-id is what its program sets, often its name in lower case. [Windows](windows.md#rules) covers how rules play out.

## Scratchpads

```
scratchpad <name> <app-id> <command>
```

A scratchpad is a floating window that `scratchpad <name>` shows over whatever you're doing and hides again. It's the first window with the app-id; when there is none, the command starts one, so the command has to give its window that app-id. Defining any scratchpad replaces the default one, a terminal on Super+Return:

```
scratchpad term foot-scratch foot --app-id foot-scratch
```

## Input devices

```
input <type|name-glob> <option> <value>...
```

An input line sets libinput options on the input devices it matches, such as tap-to-click on a touchpad. Without one, devices keep libinput's defaults, which leave tapping and natural scrolling off. The first word matches devices by type, or, if it isn't a type, by name, as a pattern like those of rules:

| Type | Devices |
|------|---------|
| `keyboard` | Keyboards. |
| `pointer` | Mice, trackballs and trackpoints: pointers other than touchpads. |
| `touchpad` | Touchpads: pointers that can tap. |
| `touch` | Touchscreens. |
| `tablet` | Drawing tablets. |

Options:

| Option | Values | Effect |
|--------|--------|--------|
| `tap` | `on`, `off` | Tap to click: one finger for left, two for right, three for middle. |
| `drag` | `on`, `off` | Tap and then hold to drag. |
| `natural-scroll` | `on`, `off` | Scroll the content, as on a phone, instead of the view. |
| `dwt` | `on`, `off` | Disable the touchpad while typing. |
| `left-handed` | `on`, `off` | Swap the left and right buttons. |
| `middle-emulation` | `on`, `off` | Pressing left and right together is a middle click. |
| `accel-profile` | `flat`, `adaptive` | Pointer acceleration: none, or faster movement goes further. |
| `accel-speed` | `-1` to `1` | Pointer speed; `0` is the device's normal speed. |

Every matching line applies, in order, so a later line can change what an earlier one set. An option no line sets keeps the device's default, and an option a device doesn't have is skipped:

```
input touchpad tap on natural-scroll on dwt on
input pointer accel-profile flat
input "Logitech G502*" accel-speed -0.3
```

Device names are what the hardware calls itself, such as `SynPS/2 Synaptics TouchPad`; `swm -log-level debug` logs each device's name and type as it appears. river can only change libinput options when it runs on the hardware, not nested in another session.

Key repeat is set for every keyboard by the `repeat-delay` and `repeat-rate` settings, not by input lines.

## Commands

Bindings run commands, and so does `streamctl`. Numbers with a sign change the current value (`mfact +0.05`); numbers without one set it (`mfact 0.55`).

| Command | Action |
|---------|--------|
| `spawn <command>` | Run the rest of the line with `sh -c`. |
| `close` | Close the focused window. |
| `focus next\|prev` | Focus the next or previous window. |
| `swap next\|prev` | Swap the focused window with the next or previous one. |
| `zoom` | Make the focused window the master, or, if it is, the next one. |
| `toggle-float` | Float or tile the focused window. |
| `toggle-fullscreen` | Make the focused window fullscreen, or end it. |
| `layout tile\|monocle\|floating\|prev` | Set the layout, or go back to the previous one. |
| `mfact <n>` | Set or change the master area's share of the width. |
| `nmaster <n>` | Set or change the number of master windows, from 0 up to the tiling windows shown. |
| `gap <n>\|reset` | Set or change the gap, or go back to the configured one. |
| `view <tag>\|all\|prev` | Show one tag, all tags, or the previous tags. |
| `toggle-view <tag>` | Show or hide a tag alongside the others. |
| `tag <tag>\|all` | Move the focused window to a tag, or to all of them. |
| `toggle-tag <tag>\|all` | Add or remove a tag on the focused window. With `all`, put it on every tag, or, if it is already, back on just the shown ones. |
| `focus-mon next\|prev` | Focus the next or previous monitor. |
| `tag-mon next\|prev` | Send the focused window to the next or previous monitor. |
| `move` | Drag a floating window, or move a tiled one to another place in the layout. Pointer bindings only. |
| `resize` | Resize a floating window from its nearest corner, or drag the nearest divider between tiled windows. Pointer bindings only. |
| `reset [mfact\|weights]` | Restore the configured master size, the window heights set by dragging, or both. |
| `scratchpad <name>` | Show or hide a scratchpad. |
| `mode <name>` | Switch to a binding mode. |
| `reload` | Read the config file again. |
| `quit` | End the river session. |

[Tiling](tiling.md) and [Windows](windows.md) describe what these do in practice.

## Reloading

The `reload` command (Mod+Shift+r) reads the file again and applies its settings, bindings, rules, scratchpads and input lines. An option whose input line is gone goes back to the device's default. What you've changed by hand stays: each tag's layout, master size and master count, and each monitor's gap. `reset` and `gap reset` go back to the configured values. A scratchpad that's no longer defined becomes an ordinary window.

## Errors and warnings

`swm` checks the whole file before applying any of it.

* **Errors**, such as an unknown setting or a binding to a mode that doesn't exist, stop the file from loading. `swm` keeps the config it has, or its defaults when starting, and reports every problem with its file and line number.
* **Warnings**, such as a key bound twice without `-also`, don't stop it. The file loads, and the warnings are reported.

Both appear in `swm`'s log, in the output of `streamctl reload`, and in `streamctl watch` for a bar to show. To check a file without loading it, run:

```sh
streamctl gen-config -check
```

It also reports programs that bindings and scratchpads start but that aren't installed.

## Keeping up with the defaults

Since your bindings replace the defaults, bindings that a new version of stream adds don't appear in your config. `streamctl gen-config -missing` prints the default bindings it lacks, as lines to paste into it:

```
# Default bindings missing from /home/you/.config/stream/config.
# Paste the ones you want into it, then reload.

bind Mod+4 view 4
# Mod+Shift+0 is taken by close: pick another key
bind Mod+Shift+0 toggle-tag all
bind Super+F11 mode passthrough

mode passthrough {
	bind Super+F11 mode normal
}
```

A binding counts as present when any binding in its mode runs the same command with the same arguments, whatever its keys. A note above a line says when it needs changing first: its key is already taken, or it needs a scratchpad your config doesn't define.
