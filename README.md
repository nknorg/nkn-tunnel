# NKN Tunnel

Tunnel tcp through NKN client. Neither side needs to have public IP address.

```
peer <--> TCP <--> NKN client <--> NKN client <--> TCP <--> peer
```

## Build

```shell
go build -o nkn-tunnel bin/main.go
```

## Basic Usage

"Server" side:

```shell
./nkn-tunnel -from nkn -to 127.0.0.1:8080 -s <seed>
```

and you will see an output like `Listening at xxx` where `xxx` is the server
listening address.

"Client" side:

```shell
./nkn-tunnel -from 127.0.0.1:8081 -to <server-listening-address>
```

Now any TCP connection to client port 8081 will be forwarded to server port
8080.
