# Bars

stream has no bar of its own. Instead, `streamctl watch` reports its state as JSON, and any bar that can run a command can show it: tags, layout, the focused window, the binding mode and config problems. Clicks run `streamctl` commands.

## The state

`streamctl watch` prints one JSON object per line: once when it starts, then after every change.

```json
{"version":1,"mode":"normal","outputs":[{"name":"DP-1","focused":true,"layout":"tile","nmaster":1,"tags":[3],"occupied":[1,3],"title":"vim","app_id":"foot"}],"config_errors":[],"config_warnings":[]}
```

| Field | Meaning |
|-------|---------|
| `version` | The format's version. It changes only if a field is removed or changes meaning; new fields can appear without it. |
| `mode` | The active binding mode. |
| `outputs` | One entry per monitor, left to right. |
| `outputs[].name` | The output's name, such as `DP-1`. |
| `outputs[].focused` | Whether this is the focused monitor. |
| `outputs[].layout` | `tile`, `monocle` or `floating`. |
| `outputs[].nmaster` | The master count. It can be more than the windows shown. |
| `outputs[].tags` | The tags shown, lowest first. |
| `outputs[].occupied` | The tags that have windows. |
| `outputs[].title`, `outputs[].app_id` | The monitor's focused window, or empty. |
| `config_errors` | What was wrong with the config file the last time it was read. |
| `config_warnings` | Warnings about the config in use, such as a key bound twice. |

`streamctl watch` keeps running while `swm` doesn't. Started before `swm`, or while `swm` restarts, it waits and carries on once `swm` is back, so a bar never has to restart it. It ends when river does.

## Clicks

Bind clicks to `streamctl` commands, such as `streamctl view 3`. Commands act on the focused monitor, and the pointer focuses the monitor it's on, so clicking a tag on a monitor's bar changes that monitor. Some useful ones, as in dwm:

| Click | Command |
|-------|---------|
| A tag | `streamctl view <n>` |
| A tag, right button | `streamctl toggle-view <n>` |
| The layout | `streamctl layout prev` |
| The layout, right button | `streamctl layout monocle` |
| The title, middle button | `streamctl zoom` |

## waybar

This setup needs waybar and [jq](https://jqlang.org).

waybar runs one bar per monitor and tells each bar's scripts which monitor it's on in `WAYBAR_OUTPUT_NAME`. jq picks that monitor out of the state, and turns it into the text and class each module shows. Add custom modules to waybar's config:

```jsonc
"modules-left": [
  "custom/tag-1", "custom/tag-2", "custom/tag-3", "custom/tag-4", "custom/tag-5",
  "custom/tag-6", "custom/tag-7", "custom/tag-8", "custom/tag-9",
  "custom/layout", "custom/mode", "custom/title"
],
"modules-right": ["custom/config", "clock"],

"custom/tag-1": {
  "exec": "streamctl watch | jq --unbuffered -c '.outputs[] | select(.name == env.WAYBAR_OUTPUT_NAME) | {text: \"1\", class: (if (.tags | contains([1])) then \"focused\" elif (.occupied | contains([1])) then \"occupied\" else \"empty\" end)}'",
  "return-type": "json",
  "on-click": "streamctl view 1",
  "on-click-right": "streamctl toggle-view 1",
  "tooltip": false
},

"custom/layout": {
  "exec": "streamctl watch | jq --unbuffered -c '.outputs[] | select(.name == env.WAYBAR_OUTPUT_NAME) | {text: (if .nmaster == 1 then .layout else \"\\(.layout) \\(.nmaster)\" end)}'",
  "return-type": "json",
  "on-click": "streamctl layout prev",
  "on-click-right": "streamctl layout monocle",
  "tooltip": false
},

"custom/mode": {
  "exec": "streamctl watch | jq --unbuffered -c '{text: (if .mode == \"normal\" then \"\" else .mode end)}'",
  "return-type": "json",
  "tooltip": false
},

"custom/title": {
  "exec": "streamctl watch | jq --unbuffered -c '.outputs[] | select(.name == env.WAYBAR_OUTPUT_NAME) | {text: .title}'",
  "return-type": "json",
  "escape": true,
  "max-length": 80,
  "on-click-middle": "streamctl zoom",
  "tooltip": false
},

"custom/config": {
  "exec": "streamctl watch | jq --unbuffered -c '{text: (if (.config_errors | length) > 0 then \"config error\" elif (.config_warnings | length) > 0 then \"config warning\" else \"\" end), tooltip: (.config_errors + .config_warnings | join(\"\\n\"))}'",
  "return-type": "json"
}
```

Tags 2 to 9 are copies of tag 1 with the number changed in all four places. What each module shows:

* **Tags**: each tag's class is `focused`, `occupied` or `empty`, for the stylesheet to colour.
* **Layout**: the layout, with the master count when it isn't 1.
* **Mode**: the mode, hidden in the normal mode.
* **Title**: the focused window's title.
* **Config**: a notice when the config has errors or warnings, with the details in its tooltip.

waybar hides a module while its text is empty.

The stylesheet gives the tags their look. With the colours from [h2o](theming.md):

```css
#custom-tag-1, #custom-tag-2, #custom-tag-3, #custom-tag-4, #custom-tag-5,
#custom-tag-6, #custom-tag-7, #custom-tag-8, #custom-tag-9 { padding: 0 6px; color: #46637e; }

#custom-tag-1.occupied, #custom-tag-2.occupied, #custom-tag-3.occupied,
#custom-tag-4.occupied, #custom-tag-5.occupied, #custom-tag-6.occupied,
#custom-tag-7.occupied, #custom-tag-8.occupied, #custom-tag-9.occupied { color: #8eabc4; }

#custom-tag-1.focused, #custom-tag-2.focused, #custom-tag-3.focused,
#custom-tag-4.focused, #custom-tag-5.focused, #custom-tag-6.focused,
#custom-tag-7.focused, #custom-tag-8.focused, #custom-tag-9.focused { color: #0b1826; background: #3fa9f5; }

#custom-mode { padding: 0 8px; color: #0b1826; background: #8fe3f0; }
#custom-config { padding: 0 8px; color: #0b1826; background: #f2707a; }
```

Start waybar after `streamctl wait` in the river init, as in [Getting started](getting-started.md#the-river-init).

## Other bars

Any bar that can run a command and read its output works the same way: run `streamctl watch`, pick out the fields you need, and run `streamctl` commands on clicks. Pick out one monitor by its `name`, or show the focused one with `select(.focused)`.
