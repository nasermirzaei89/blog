# Blog

## Run

Copy `.env.example` to `.env` and update it.
Then:

```shell
go run .
```

or

```shell
npm install
make npm-build
make build
./bin/blog
```

For using forgot-password you need to run:

```shell
docker run --name=mailpit -p 8025:8025 -p 1025:1025 axllent/mailpit
```
