# Monitors

stream handles several monitors the way dwm does: each monitor has its own nine tags, layouts and gap, and shows its own windows.

## The focused monitor

One monitor is focused at a time. New windows open on it, and commands such as `view` or `layout` act on it.

* Moving the pointer onto a monitor focuses it, even over an empty part of it.
* Mod+comma and Mod+period focus the previous or next monitor. Monitors are ordered left to right, then top to bottom, and the order wraps around.
* Clicking or focusing a window focuses its monitor.

Bars rely on this: to click a monitor's bar, the pointer has to be on that monitor, which focuses it, so the tag you click changes there. See [Bars](bars.md).

## Moving windows between monitors

Mod+Shift+comma and Mod+Shift+period send the focused window to the previous or next monitor. It lands on that monitor's shown tags as its master, so it's visible right away. A floating window keeps its place relative to the monitor.

Dragging a floating window across with Mod+left drag moves it too. To open a program on a particular monitor every time, use a rule with `monitor`, naming the monitor by its output name:

```
rule -app-id firefox monitor DP-1
```

`streamctl watch` lists the output names of the connected monitors.

## Unplugging and plugging in

When a monitor is unplugged, its windows move to another monitor, onto that monitor's shown tags, so none are lost or hidden.

stream remembers each monitor by its output name. When one is plugged back in, it gets back its tags, the layout of each tag and its gap. Windows that moved away stay where they are; send them back with Mod+Shift+comma or Mod+Shift+period.

## Switching VTs

When you switch to another virtual terminal (Ctrl+Alt+F1 and so on), river disconnects every monitor until you come back. With nowhere to move the windows, stream sets them aside. When the monitors return, each window goes back to its monitor, with its tags and its place in the layout.
