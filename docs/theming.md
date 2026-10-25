# Theming

stream itself draws only window borders. A desktop's look comes from all its programs: the terminal, notifications, the bar, GTK apps. A theme is a set of colours for each of them, in files those programs read.

stream comes with one: **h2o**, clear water blue on deep water. Its border colours are stream's defaults, `make install` installs its wallpaper, and the colours for other programs are in [examples/h2o](../examples/h2o). This page shows how to use it and how to make your own from it.

## How a theme is laid out

Each program's colours go in a file of their own, next to the program's config, and the config includes that file:

```
~/.config/foot/foot.ini        your foot config
    include=~/.config/foot/theme.ini
~/.config/foot/theme.ini       the theme's colours for foot
```

Your config stays yours: fonts, keys and everything else live in it, and the theme file holds only colours. Changing theme means putting other colour files in place, without touching any config.

Colours that several programs use are kept once, in `~/.config/theme/`: `colors.css` for GTK and waybar, and `colors.sh` for shell scripts.

## Using h2o

1. Copy the theme's files into your home directory:

   ```sh
   cp -r examples/h2o/home/. ~/
   ```

   The files have names of their own, such as `theme.ini`, so this doesn't overwrite your configs.

2. Add the include line to each program's config. The [h2o README](../examples/h2o/README.md#files) lists the line for each program:

   ```
   ~/.config/stream/config     include -optional ~/.config/stream/theme
   ~/.config/foot/foot.ini     include=~/.config/foot/theme.ini
   ~/.config/mako/config       include=~/.config/mako/theme
   ~/.config/tmux/tmux.conf    source-file -q ~/.config/tmux/theme.conf
   ```

   In GTK's `gtk.css` and waybar's `style.css`, `@import url("../theme/colors.css");` makes the palette's colours available by name, such as `@accent`.

3. Draw the wallpaper from the river init:

   ```sh
   swaybg -i /usr/local/share/backgrounds/stream/h2o.png -m fill &
   ```

4. Reload each program (see [Switching themes](#switching-themes)), or log out and back in.

## Making a theme

### 1. Start from h2o

```sh
cp -r examples/h2o ~/my-theme
```

### 2. Choose a palette

A theme needs a handful of colours, each with a role. h2o's:

| Role | h2o | Used for |
|------|-----|----------|
| `base` | `#0b1826` | Backgrounds: bars, notifications, menus |
| `shade` | `#132536` | Raised surfaces: selections, the title in the bar |
| `line` | `#1f3a52` | Borders, separators, unfocused windows |
| `text` | `#e6f1fa` | Text |
| `dim` | `#8eabc4` | Secondary text |
| `accent` | `#3fa9f5` | Focus: the focused window, the shown tags, the cursor |
| `accent2` | `#8fe3f0` | A second accent, for things that need to stand apart |
| `urgent` | `#f2707a` | Errors, urgent notifications |
| `success` | `#6fd6a8` | Success |

Some guidance:

* Keep `text` readable on `base` and on `shade`, and the text on an `accent` background readable too (h2o uses `base` for it).
* Make `line` close to `base`, so unfocused borders recede and the focused one stands out.
* Terminals also need 16 ANSI colours (`foot/theme.ini`). Derive them from the palette: red from `urgent`, green from `success`, blue from `accent`, and so on.
* Keep the terminal background `#000000` if you use terminal transparency. The theme then only changes the colours drawn on it.

### 3. Put the colours in the files

Every file under `~/my-theme/home` uses the palette's hex values. Replace each h2o colour with yours. With `sed`, for each colour:

```sh
grep -rl 3fa9f5 ~/my-theme/home | xargs sed -i 's/3fa9f5/e0a040/g'
```

Then check the comments and terminal colours by eye. The one file stream reads, `.config/stream/theme`, is just two lines:

```
border-focused #e0a040
border-unfocused #2a2a2a
```

### 4. Draw a matching wallpaper

`examples/h2o/wallpaper` is the program that draws h2o's wallpaper from its palette: a flat night landscape with a river. Change the colours at the top of `main.go` to yours and run it:

```sh
cd ~/my-theme/wallpaper
go run . -o ~/my-theme/wallpaper.png -w 3840 -h 2400
```

Make it at least as large as your largest monitor. Any other image works as a wallpaper too.

### 5. Install it

As with h2o: copy `home/` into your home directory, point swaybg at the wallpaper, and reload.

## Switching themes

Put the new theme's files in place, then reload what's running:

| Program | Reload |
|---------|--------|
| stream | Mod+Shift+r, or `streamctl reload` |
| foot | New windows use the new colours |
| mako | `makoctl reload` |
| tmux | `tmux source-file ~/.config/tmux/tmux.conf` |
| waybar | `pkill -USR2 waybar` |
| swaybg | Stop it and start it with the new image |
| dunst, GTK apps | Restart them |
