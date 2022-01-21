# wl-gammarelay (alpha)

This utility was developed from `gammastep`, a fork of `redshift` as well
as examples from `wlroots`.

It allows users to use keybindings to dynamically change the color temperature
and brightness.

This used to be possible using redshift, when using the `-P -O <temp>` flags,
but since wayland requires the client to keep running, I developed this tool
that reads from stdin and dynaimcally updates the temperature and/or
brightness.

The first invocation to `wl-gammarelay` will spin up a daemon and set the
desired temperature.

All other invocations will send requests via unix domain socket.

Below are some examples on how this utility can be used to change the color
temperature via keybindings in `swaywm`:

```
bindsym $mod+Control+Minus      exec wl-gammarelay -t -100
bindsym $mod+Control+Equal      exec wl-gammarelay -t +100
bindsym $mod+Control+0          exec killall wl-gammarelay
bindsym $mod+Control+Underscore exec wl-gammarelay -b -0.02
bindsym $mod+Control+Plus       exec wl-gammarelay -b +0.02
```

Tested on Arch Linux using swaywm.

## Installation

```
make
sudo make install
```

To install to `/usr/bin` set the `PREFIX` variable when running `make`:

```
sudo make install PREFIX=/usr
```

## Dependencies

- go 1.17
- wayland
