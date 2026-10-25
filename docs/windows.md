# Windows

## Focus

The focused window has the `border-focused` colour and gets the keyboard.

* **Hovering** focuses: when the pointer moves onto a window, that window takes focus. A window that slides under a still pointer, because the layout changed, doesn't.
* **Clicking** focuses a window and raises it if it floats.
* **Keys** move focus too: Mod+j and Mod+k within a monitor, Mod+comma and Mod+period between monitors.
* **Closing** a window gives focus back to the window focused before it on that monitor.
* **New windows** take focus when they open where you're looking. A window that a rule sends to a hidden tag or another monitor opens there without taking focus.
* **A fullscreen window** keeps focus: Mod+j and Mod+k don't move away from it.

stream never moves the pointer for you.

Launchers, lock screens and other overlays take the keyboard while they're open, and give it back when they close.

## The mouse

Hold Mod and use the pointer:

| | Floating window | Tiled window |
|-|-----------------|--------------|
| Mod+left drag | Moves it. Its edges snap to the screen's edges when close. | Moves it to another place in the layout: the windows it passes shift to make room. |
| Mod+right drag | Resizes it from the corner nearest the pointer; the opposite corner stays put. | Drags the nearest divider: the one between the columns, or the one between two windows in a column. This works from the gap between windows too. |
| Mod+middle click | Tiles it. | Floats it. |

A drag acts on the window under the pointer and focuses it first. Dragging a floating window onto another monitor moves it to that monitor.

Programs that draw their own title bar can be moved and resized by it, as usual, while they float. Tiled windows stay where the layout puts them.

## Floating windows

A window floats when:

* it's a dialog, which opens centred over the window it belongs to;
* it has a fixed size, as some utilities and games do;
* a rule says `float`;
* you float it with Mod+Shift+space or Mod+middle click;
* its tag uses the floating layout.

Floating windows stay above tiled ones, and the last one clicked or opened is on top. A floating window with no place yet opens centred on its monitor. A tiled window you float stays where it was, at its tiled size, until you move it.

## Fullscreen

A program can ask to go fullscreen, as a video player does, and Mod+Shift+f toggles it for the focused window. A fullscreen window fills its whole monitor, above the other windows.

## Borders and title bars

stream asks every program to leave out its title bar, so windows show only a border. Some programs draw one anyway.

## Rules

Rules ([syntax](configuration.md#rules)) apply once, when a window opens:

* `float` and `tile` decide whether it floats.
* `tag` opens it on other tags, and `monitor` on another monitor. It doesn't take focus there.
* Every matching rule applies in order, so a later rule can override an earlier one.

To find a window's app-id and title, focus it and run `streamctl watch`: the focused monitor's `app_id` and `title` are the focused window's.

## Scratchpads

A scratchpad ([syntax](configuration.md#scratchpads)) is a window you call up and put away with one key, such as a terminal on Super+Return.

* **The first press** starts its command. The window opens floating, centred on the focused monitor.
* **Later presses** hide it, or show it again, over the tags you're viewing on the focused monitor, focused and on top.
* **Hiding it** gives focus back to the window you were using.

The scratchpad is the first window with its app-id. Close it and the next press starts a new one. Rules don't apply to it.
