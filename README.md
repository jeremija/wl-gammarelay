# wl-gammarelay

This utility was developed from `gammastep`, a fork of `redshift` as well
as examples from `wlroots`.

It allows using keybindings to dynamically change the color temperature
and software brightness.

This used to be possible using redshift using the `-P -O <temp>` flags, but
since wayland requires the client to keep running, I developed this tool
that spins up a daemon and can be controlled via unix domain socket.

The first invocation to `wl-gammarelay` will spin up a daemon. This can be
disabled using the `--no-daemon/-D` flag, but if the daemon isn't already running
in the background the requests will fail. After the daemon starts up, the
temperature and brightess will be set to the desired levels.

The daemon will also write the last color temperature and brightness to a
history file which can then be tailed to display the value(s) in `waybar` or
`i3status-rust`. The path can be set using the `--history/-H` flag, which should
be set to an empty string to disable this functionality.

All other invocations act as clients only send requests via unix domain socket.
The path of the socket for both the daemon and the client can be controlled
using the `--sock/-s` flag.

The `--temperature/-t` flag accepts an integer in the range of `[1000, 10000]`,
when set to an absolute values. Relative changes can be specified by adding a
`+` or `-` prefix before the integer.

The `--brigtness/-b` flag behaves similarly to temperature, only its range is
`[0, 1.0]` and it accepts floats.

Below are some examples on how this utility can be used to change the color
temperature via keybindings in `swaywm`:

```config
bindsym $mod+Control+Minus      exec wl-gammarelay -t -100
bindsym $mod+Control+Equal      exec wl-gammarelay -t +100
bindsym $mod+Control+0          exec killall wl-gammarelay
bindsym $mod+Control+Underscore exec wl-gammarelay -b -0.02
bindsym $mod+Control+Plus       exec wl-gammarelay -b +0.02
```

Sample configuration for `waybar`:

```config
"modules-right": ["custom/wl-gammarelay"],
"custom/wl-gammarelay": {
  "format": "{} ",
  "exec": "tail -F /tmp/.wl-gammarelay.hist 2>/dev/null"
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

## Unix Domain Socket Protocol

The default path of the unix socket will be set to
`$XDG_RUNTIME_DIR/wl-gammarelay.sock`.

The daemon expects a JSON message terminated by a newline `\n` character.
Multiple simultaneous connections to the service are possible, but the daemon
will handle each request one by one.

The daemon currently only only writes the temperature updates to the connection
that made the request, but this might change in the future. For example, we
might want to enable sending updates to all other connections so that
applications that are interested can update the UI.

The [types](types/) folder contains all the type definitions used by the
protocol.

The clients send a `types.Request` and the server will respones with a
`types.Response`.

The `-v` flag can be used to enable logging of requests and responses in both
the daemon and the client.

Some examples:

```console
$ wl-gammarelay -v -t 4000 -b 0.8
{"color":{"temperature":"4000","brightness":"0.8"}}
{"color":{"temperature":"4000","brightness":"0.8"}}

$ wl-gammarelay -v -t 4000
{"color":{"temperature":"4000"}}
{"color":{"temperature":"4000","brightness":"0.8"}}

$ wl-gammarelay -v -t +100
{"color":{"temperature":"+100"}}
{"color":{"temperature":"4100","brightness":"0.8"}}

$ wl-gammarelay -v -t -100
{"color":{"temperature":"-100"}}
{"color":{"temperature":"4000","brightness":"0.8"}}
```

## Dependencies

- go 1.17
- wayland (libwayland-dev)
