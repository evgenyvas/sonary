Sonary
======

My local sound library

Install
=======

Compile

```
  go mod download
  make build
```

Create configuration file `.env.local` and set APP_ENV parameter. If `.env.local` is not present, will be used default parameters from **.env**. All parameters from `.env.local` will get priority over parameters from all other config files.

- APP_ENV - which environment specific config file will be loaded, either **dev** or **prod**
- HOST - which host will be used by server
- ROOT_PATH - path to directory to store your music

Start server:

```
  ./sonary
```

Build frontend:

```
cd ./frontend
yarn install
yarn build
```

If **HOST** paramenter in config file equals `:8080`, WEB interface will be available by address http://localhost:8080

Or you can install systemd service, use example in `support/systemd/sonary.service`
