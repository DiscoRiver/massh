### For testing continuous streams, generate continuous output
hexdump -C /dev/urandom | GREP_COLOR='1;32' grep --color=auto 'ca fe'