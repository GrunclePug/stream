# Tiling

stream arranges windows the way dwm does: one main window takes a large share of the screen, and the rest share what's left. You rarely place a window yourself.

## The tile layout

The screen splits into two columns. The **master** area on the left holds the newest window; the **stack** on the right holds the others, one above the other.

```
+-------------+------+
|             |  2   |
|             +------+
|      1      |  3   |
|             +------+
|             |  4   |
+-------------+------+
```

* **New windows** become the master and push the others down the stack.
* **Mod+Return** (`zoom`) makes the focused window the master. On the master itself, it brings up the next window instead, so pressing it twice toggles between the two.
* **Mod+j / Mod+k** focus the next or previous window, and **Mod+Shift+j / Mod+Shift+k** swap the focused window with it, wrapping around at the ends.
* **Mod+h / Mod+l** shrink or grow the master area (`mfact`), from 5% to 95% of the width.
* **Mod+i / Mod+d** put more or fewer windows in the master area (`nmaster`). The count goes from 0, where every window stacks, up to the number of windows tiled. If windows close, the count you set is kept for when they come back, but Mod+d always steps down from what's on screen.

Windows in a column share its height evenly. Dragging the divider between two of them with Mod+right drag changes how they share it; `reset weights` evens them out again. Dragging the divider between the columns sets `mfact`. See [Windows](windows.md#the-mouse).

## Gaps

One gap separates windows from each other and from the screen edges. Mod+minus and Mod+equal shrink and grow it, Mod+Shift+equal goes back to the configured size, and `gap 0` removes it. Each monitor has its own gap.

## Monocle

Mod+m. Every tiled window fills the screen, and the focused one is on top. Mod+j and Mod+k move between them.

## Floating

Mod+f. Nothing tiles: every window keeps its own size and place, and the mouse moves and resizes them. Windows that were tiled stay where they were. Floating windows in the other layouts are covered in [Windows](windows.md#floating-windows).

Mod+space switches back to the previous layout, so Mod+m then Mod+space goes from monocle back to tile.

## Tags

Each monitor has nine tags. They work like workspaces, with two differences:

* **A window can be on several tags.** Mod+Ctrl+Shift+3 adds tag 3 to the focused window, or removes it. Mod+Shift+0 puts the window on every tag, and pressing it again puts it back on just the tags you're viewing.
* **You can view several tags at once.** Mod+Ctrl+3 adds tag 3 to what's shown, or removes it, and Mod+0 shows every tag.

Mod+1..9 view a tag, Mod+Shift+1..9 move the focused window to one, and Mod+Tab goes back to the tags shown before. Viewing the tags already shown does nothing.

### Per-tag layouts

Each tag remembers its own layout, master size and master count. With several tags shown, the lowest one's apply. So tag 5 can stay in monocle while the others tile.

## The layout in a bar

`streamctl watch` reports each monitor's layout, master count and tags, so a bar can show them and switch tags on a click. See [Bars](bars.md).
