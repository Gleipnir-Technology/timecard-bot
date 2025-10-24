# Timecarder

A Matrix bot for doing timecard stuff.

## Building

On NixOS:

```
nix build .
```

This will produce files in `result/`, including `result/bin/timecardbot`

## Configuring

You'll need to provide various options

### POSTGRES_URI

This is the connection URI for the database you'll be using. If you use unix domain sockets I'd recommend something like `postgres:///dbname?host=/var/run/postgresql`

## Hacking

On NixOS:

```
nix develop
```

### bob

This uses the bob query framework. You can regenerate the models for bob with:

```
PSQL_DSN="postgresql://dbname?host=/var/run/postgresql&sslmode=disable" go run github.com/stephenafamo/bob/gen/bobgen-psql@latest"
```

This will generate a bunch of files.
