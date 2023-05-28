# Massh V2

## Introduction

## Testing

Testing SSH packages requires running `setup_ssh_server.sh` which will build a container image from `Dockerfile` that overrides localhost. We can then perform SSH tasks in a sandbox with `test:test` credentials, without exposing our _actual_ host machine.

It's a terrible solution that relies on a dev's local configuration, but it's the most painless way I've come across to do SSH testing outside of production or a sandbox cluster. In those cases IP lookup and such is necessary, and this way reduces the variables down dramatically. I would welcome a better solution.