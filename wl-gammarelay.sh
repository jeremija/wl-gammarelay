#!/bin/bash

set -eu

cmd=wl-gammarelay
input_file="$HOME/.wl-gammarelay.input"
hist_file="$HOME/.wl-gammarelay.hist"

touch $hist_file
touch $input_file

if ! pgrep \^$cmd\$; then
  echo No process found, starting...
  (
    ( tail -n1 $hist_file && tail -n0 -f $input_file ) | $cmd | while read line; do
      echo read $line
      echo $line > $hist_file
    done
  ) &
else
  echo Process already running...
fi

if [[ "$#" -eq 0 ]]; then
  exit 0
fi

temperature=${1:-6500}
brightness=${2:-+0}

# Truncate the file. If we jus did echo $temperature $brightness > $input_file
# it wouldn't be recognized by the process.
cat /dev/null > $input_file
echo $temperature $brightness >> $input_file
