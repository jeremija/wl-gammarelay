# wl-gammarelay

This utility was developed from `gammastep`, a fork of `redshift` as well
as examples from `wlroots`.

It allows using keybindings to dynamically change the color temperature
and software brightness.

This used to be possible using redshift using the `-P -O <temp>` flags, but
since wayland requires the client to keep running, I developed this tool
that spins up a daemon and can be controlled via unix domain socket.

The `wl-gammarelay` is a daemon which listens to DBus requests:

```
busctl --user -- call rs.wl-gammarelay / rs.wl.gammarelay UpdateTemperature n -500
busctl --user -- call rs.wl-gammarelay / rs.wl.gammarelay UpdateTemperature n +500

busctl --user -- call rs.wl-gammarelay / rs.wl.gammarelay UpdateBrightness d -0.2
busctl --user -- call rs.wl-gammarelay / rs.wl.gammarelay UpdateBrightness d +0.2

busctl --user -- set-property rs.wl-gammarelay / rs.wl.gammarelay Brightness d 0.5
busctl --user -- set-property rs.wl-gammarelay / rs.wl.gammarelay Brightness d 1

busctl --user -- set-property rs.wl-gammarelay / rs.wl.gammarelay Temperature q 4000
busctl --user -- set-property rs.wl-gammarelay / rs.wl.gammarelay Temperature q 6500
```

The service can be introspected:

```
$ busctl --user introspect rs.wl-gammarelay / rs.wl.gammarelay
NAME               TYPE      SIGNATURE RESULT/VALUE FLAGS
.UpdateBrightness  method    d         -            -
.UpdateTemperature method    n         -            -
.Brightness        property  d         1            emits-change writable
.Temperature       property  q         6500         emits-change writable
```

The `UpdateBrightness` and `UpdateTemperature` are here for relative
adjustments.

Sample hotkey configuration for `sway`:

```config
exec wl-gammarelay

bindsym $mod+Control+Minus      exec busctl --user -- call rs.wl-gammarelay / rs.wl.gammarelay UpdateTemperature n -100
bindsym $mod+Control+Equal      exec busctl --user -- call rs.wl-gammarelay / rs.wl.gammarelay UpdateTemperature n +100
bindsym $mod+Control+0          exec busctl --user set-property rs.wl-gammarelay / rs.wl.gammarelay Brightness d 1.0 && busctl --user set-property rs.wl-gammarelay / rs.wl.gammarelay Temperature q 6500
bindsym $mod+Control+Underscore exec busctl --user -- call rs.wl-gammarelay / rs.wl.gammarelay UpdateBrightness d -0.02
bindsym $mod+Control+Plus       exec busctl --user -- call rs.wl-gammarelay / rs.wl.gammarelay UpdateBrightness d +0.02
```

Sample configuration for `waybar`:

```json
{
  "modules-right": ["custom/wl-gammarelay-temperature", "custom/wl-gammarelay-brightness"],
  "custom/wl-gammarelay-temperature": {
    "format": "{} ",
    "exec": "wl-gammarelay --subscribe Temperature",
    "on-scroll-up": "busctl --user -- call rs.wl-gammarelay / rs.wl.gammarelay UpdateTemperature n +100",
    "on-scroll-down": "busctl --user -- call rs.wl-gammarelay / rs.wl.gammarelay UpdateTemperature n -100"
  },
  "custom/wl-gammarelay-brightness": {
    "format": "{} ",
    "exec": "wl-gammarelay --subscribe Brightness",
    "on-scroll-up": "busctl --user -- call rs.wl-gammarelay / rs.wl.gammarelay UpdateBrightness d +0.02",
    "on-scroll-down": "busctl --user -- call rs.wl-gammarelay / rs.wl.gammarelay UpdateBrightness d -0.02"
  }
}
```

Tested on Arch Linux and Ubuntu 20.04 LTS using `swaywm`.

## Installation

```
make
sudo make install
```

To install to `/usr/bin` set the `PREFIX` variable when running `make`:

```
sudo make install PREFIX=/usr
```

### Arch Linux

This package is also on AUR: https://aur.archlinux.org/packages/wl-gammarelay/

## Dependencies

- go 1.17
- wayland (libwayland-dev)
