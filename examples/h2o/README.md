# h2o

stream's own look: clear water blue on deep water. Its border colours are stream's defaults, and its wallpaper is installed with stream. This directory holds the rest of the theme as an example: copy it if you like it, or use it as a model for your own.

![h2o wallpaper](wallpaper.png)

## Palette

| Name | Colour | Used for |
|------|--------|----------|
| `base` | `#0b1826` | Backgrounds |
| `shade` | `#132536` | Raised surfaces: bars, selections |
| `line` | `#1f3a52` | Borders, separators, unfocused windows |
| `text` | `#e6f1fa` | Text |
| `dim` | `#8eabc4` | Secondary text |
| `accent` | `#3fa9f5` | Focus: the focused window, the shown tags |
| `accent_bright` | `#7cc4fa` | The accent, lighter |
| `accent2` | `#8fe3f0` | A second accent |
| `urgent` | `#f2707a` | Urgent and critical |
| `success` | `#6fd6a8` | Success |

Terminals keep a black background (`#000000`), so terminal transparency works the same with every theme.

## Files

`home/` mirrors your home directory. Each app's colours sit next to its own config, as a separate file its config includes, so the theme never replaces a config you wrote:

| File | App | Add to the app's config |
|------|-----|-------------------------|
| `.config/stream/theme` | stream | `include -optional ~/.config/stream/theme` |
| `.config/foot/theme.ini` | foot | `include=~/.config/foot/theme.ini` in `[main]` |
| `.config/mako/theme` | mako | `include=~/.config/mako/theme` |
| `.config/tmux/theme.conf` | tmux | `source-file -q ~/.config/tmux/theme.conf` |
| `.config/dunst/dunstrc.d/50-theme.conf` | dunst | Nothing: dunst reads its drop-in directory |
| `.config/theme/colors.css` | GTK 3 and 4, waybar | `@import url("../theme/colors.css");` in `gtk.css` or waybar's `style.css`, then `@base`, `@accent` and so on |
| `.config/theme/colors.sh` | Shell scripts | `. ~/.config/theme/colors.sh`, then `$base`, `$accent` and so on |

`colors.css` and `colors.sh` hold the whole palette, for anything several apps share. The other files only set their app's colours.

To install everything, copy the tree into your home directory and add the include lines:

```sh
cp -r examples/h2o/home/. ~/
```

The files have new names, so the copy doesn't overwrite any config you already have.

## Wallpaper

`make install` installs the wallpaper as `$(PREFIX)/share/backgrounds/stream/h2o.png`, which the init example in stream's README draws. It's flat art, 3840×2400, so it fills 16:10 and 16:9 screens up to 4K.

`wallpaper/` is the program that draws it from the palette. To make a wallpaper for your own theme, change the colours at the top of `wallpaper/main.go` and run it:

```sh
cd examples/h2o/wallpaper
go run . -o ~/wallpaper.png -w 3840 -h 2400
```

## Making your own theme

Copy this directory, change the colours in every file, and keep the file names and include lines. Switching themes is then just putting another theme's files in place and reloading each app.
