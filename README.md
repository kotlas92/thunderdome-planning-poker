# Thunderdome Planning Poker

Thunderdome is an open source agile planning poker tool in the theme of Battling for points that helps teams estimate stories.

- Planning Sessions are **Battles**
- Users are **Warriors**
- Stories are **Plans**

### **Uses WebSockets and [Svelte](https://svelte.dev/) frontend framework for a truly Reactive UI experience**

![image](https://user-images.githubusercontent.com/846933/58061038-58d62d00-7b42-11e9-9679-ebd297a51c05.png)


## Building and running with docker-compose (easiest solution)

Prefered way of building and running the application with Postgres DB

```
docker-compose up --build
```

## Building

To run without docker you will need to first build, then setup the postgres DB,
and pass the user, pass, name, host, and port to the application as environment variables

```
DB_HOST=
DB_PORT=
DB_USER=
DB_PASS=
DB_NAME=
```

### Install dependencies
```
go get -d -v
go get -u github.com/gobuffalo/packr/packr
npm install
```

## Build with Make
```
make build
```
### OR manual steps

### Build static assets
```
npm run build
```

### bundle up static assets
```
packr
```

### Build for current OS
```
go build
```

# Let the Pointing Battles begin!

Run the server and visit [http://localhost:8080](http://localhost:8080)