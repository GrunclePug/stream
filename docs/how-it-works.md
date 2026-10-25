# How it works

## river and swm

river 0.5 is a Wayland compositor that doesn't manage windows itself. It draws the screen, handles input devices and talks to programs, but it leaves every window-management decision to a separate program, the window manager, over its `river-window-management-v1` protocol. stream's window manager is `swm`.

The work divides like this:

| river | swm |
|-------|-----|
| Draws windows, borders and layer surfaces such as bars | Decides where windows go and how big they are |
| Reads keyboards and pointers | Decides which keys are bindings, and what they do |
| Gives programs the keyboard | Decides which window has focus |
| Keeps windows open | Remembers which tags and monitors they're on |

`swm` is an ordinary Wayland client, written in Go and built without cgo.

## Manage and render sequences

river asks `swm` for decisions in two kinds of rounds:

* In a **manage sequence**, river reports what changed: new or closed windows, key and pointer presses, monitors added or removed. `swm` answers with window management state: sizes, focus, fullscreen, which bindings are on.
* In a **render sequence**, after programs have drawn at their new sizes, `swm` sets what's seen: positions, borders, stacking, and which windows are hidden.

river applies each sequence's answers at once, so a layout change appears in a single frame, without windows jumping or showing at the wrong size. Everything `swm` does happens in these rounds: key presses, `streamctl` commands and repeat timers all wait for the next manage sequence.

## Restarts

When `swm` exits, river keeps every window open, and the next window manager to connect gets them all back. The river init runs `swm` in a loop for this reason: a crash, or `pkill -x swm` to run a new build, costs about a second.

To put each window back where it was, `swm` saves its state whenever it changes:

* for each window: its monitor, tags, floating size and place, height in its column, and whether it's fullscreen or a scratchpad;
* for each monitor: its shown tags, each tag's layout, master size and master count, its gap, and the order of its windows.

Windows are matched by an identifier river gives each one, and monitors by their output name. The file is `$XDG_RUNTIME_DIR/stream-<display>.state.json`; it lives only as long as the login, and the `quit` command removes it.

## The control socket

`swm` listens on `$XDG_RUNTIME_DIR/stream-<display>.sock`, where `<display>` is the session's `WAYLAND_DISPLAY`, so each river session has its own. The protocol is lines of text:

* A client sends one line, a command such as `view 3`. It gets back any warnings, one `warning: <message>` line each, then `ok` or `error: <message>`.
* A client that sends `watch` instead receives the state as a JSON line now and after every change, until it disconnects.

`streamctl` is a client of this socket, and so is anything else that speaks it, such as a one-line `socat` script.

## What stream leaves to other programs

* **The bar, wallpaper, launcher and notifications**: separate programs, started from the river init.
* **Keyboard layouts and input devices**: not configured by stream.
* **Lock screens**: river's session lock. While it's on, only bindings marked `-locked` work.
* **Window decorations**: stream asks programs to leave out their title bars and draws only borders.
