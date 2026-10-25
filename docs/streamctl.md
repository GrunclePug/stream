# streamctl

`streamctl` controls a running `swm`, and writes and checks config files.

```
streamctl <command> [arguments]
```

## Running commands

Every [command](configuration.md#commands) a binding can run works from `streamctl`, except `move` and `resize`, which follow the pointer:

```sh
streamctl view 3
streamctl layout monocle
streamctl spawn foot
streamctl mode passthrough
streamctl reload
```

Commands act on the focused monitor and window, as a key press would. This makes `streamctl` the way to control stream from scripts, bars and other key daemons.

## watch

```sh
streamctl watch
```

Prints `swm`'s state as one JSON object per line: once at the start, then after every change. It waits for `swm` when it isn't running, and keeps going across `swm` restarts, until river exits. [Bars](bars.md#the-state) describes the format.

## wait

```sh
streamctl wait
```

Returns once `swm` is running. The river init uses it to start bars and wallpapers only once a window manager is there to show them. It fails if river exits first.

## gen-config

```
streamctl gen-config [-force] [-o [path]]
streamctl gen-config -missing [-o path]
streamctl gen-config -check [-o path]
```

Without flags, writes stream's built-in defaults, with comments, to the config file `swm` reads (`$XDG_CONFIG_HOME/stream/config`, or `~/.config/stream/config`). An existing file is left alone.

| Flag | Effect |
|------|--------|
| `-force` | Overwrite an existing file. |
| `-o path` | Use that file instead. A directory gets a file named `config` in it. |
| `-o` | Print the defaults instead of writing a file. |
| `-missing` | Print the default bindings the config lacks, as bind lines to paste into it. See [Configuration](configuration.md#keeping-up-with-the-defaults). |
| `-check` | Report the config's errors and warnings, and programs it starts that aren't installed, without loading it. |

## version

```sh
streamctl version
```

Prints the version.

## Exit status

| Status | Meaning |
|--------|---------|
| 0 | Done. Warnings, such as those `reload` or `-check` find, are printed but still exit 0. |
| 1 | The command failed: `swm` isn't running, the command couldn't run, or the config has errors. |
| 2 | The arguments were wrong: an unknown command or flag. |

## How it finds swm

`streamctl` talks to `swm` through a socket in `$XDG_RUNTIME_DIR`, named after the session's `WAYLAND_DISPLAY`. Both variables are set for everything river starts, so `streamctl` works from any terminal, script or bar inside the session. Outside it, such as over ssh, set them to the session's values first.
