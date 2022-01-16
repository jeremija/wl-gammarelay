#!/bin/bash

set -xeu

cmd=wl-gammarelay
input_file="$HOME/.wl-gammarelay.input"
hist_file="$HOME/.wl-gammarelay.hist"

touch $hist_file
touch $input_file

if ! pgrep \^$cmd\$; then
  echo No process found, starting...
  ( tail -n1 $hist_file && tail -n0 -f $input_file ) | $cmd | tee $hist_file
else
  echo Process already running...
fi

if [[ "$#" -eq 0 ]]; then
  exit 0
fi

temperature=${1:-6500}
brightness=${2:-+0}

echo $temperature $brightness >> $input_file
