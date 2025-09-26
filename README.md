# FullStack Go

## Run

Copy `.env.example` to `.env` and update it.

For the first time run:

```shell
make dep
```

To run application:

```shell
make run
```

For using forgot-password you need to run:

```shell
docker run --name=mailpit -p 8025:8025 -p 1025:1025 axllent/mailpit
```

## Test

```shell
make test
```

to debug playwright tests set `PWDEBUG=1`
