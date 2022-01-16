# wl-gammarelay (alpha)

This utility was developed from `gammastep`, a fork of `redshift` as well
as examples from `wlroots`.

It allows users to use keybindings to dynamically change the color temperature
and brightness.

This used to be possible using redshift, when using the `-P -O <temp>` flags,
but since wayland requires the client to keep running, I developed this tool
that reads from stdin and dynaimcally updates the temperature and/or
brightness.

The stdin accepts the values in the following format:

```
6500
6500 1.0
+100
-100
+0 0.8
+0 -0.02
+0 +0.02
```

Here are the explanations:

```
6500      # set the color temperature to 6500, leave brightness unchanged.
6500 1.0  # set the color temperature to 6500 and brightness to 1.0
+100      # increase the color temperature by 100, leave brightness unchanged.
-100      # decrease the color temperature by 100, leave brightness unchanged.
+0 0.8    # set the brightness to 0.8, leave color temperature unchanged.
+0 +0.02  # increase the brightness by 0.02, leave color temperature unchanged.
+0 -0.02  # decrease the brightness by 0.02, leave color temperature unchanged.
```

There is a helper script `wl-gammarelay.sh` that allows easy binding in
scripts, for example:

```
bindsym $mod+Control+Minus      exec wl-gammarelay.sh -100
bindsym $mod+Control+Equal      exec wl-gammarelay.sh +100
bindsym $mod+Control+0          exec killall wl-gammarelay
bindsym $mod+Control+Underscore exec wl-gammarelay.sh +0 -0.02
bindsym $mod+Control+Plus       exec wl-gammarelay.sh +0 +0.02
```

Tested on Arch linux using swaywm.

## Installation

```
make
sudo make install
```

## Dependencies

- wlroots (wlr, libwlroots-dev)
- wayland
