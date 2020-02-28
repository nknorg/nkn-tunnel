# NKN Tunnel [![GitHub license](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](https://github.com/nknorg/nkn-tunnel/blob/master/LICENSE) [![Go Report Card](https://goreportcard.com/badge/github.com/nknorg/nkn-tunnel)](https://goreportcard.com/report/github.com/nknorg/nkn-tunnel) [![Build Status](https://travis-ci.org/nknorg/nkn-tunnel.svg?branch=master)](https://travis-ci.org/nknorg/nkn-tunnel) [![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](#contributing)

![nkn](logo.png)

Tunnel any TCP applications through NKN client or Tuna. A few advantages:

* Network agnostic: Neither sender nor receiver needs to have public IP address
  or port forwarding. NKN tunnel only establish outbound (websocket)
  connections, so Internet access is all they need on both side.

* Top level security: All data are end to end authenticated and encrypted. No
  one else in the world except sender and receiver can see or modify the content
  of the data. The same public key is used for both routing and encryption,
  eliminating the possibility of man in the middle attack.

* Decent performance: By aggregating multiple overlay paths concurrently, one
  can get ~100ms end to end latency and 10+mbps end to end throughput between
  international devices using the default NKN client mode, or much lower latency
  and higher throughput using Tuna mode.

* Everything is open source and decentralized. The default NKN client mode is
  free (If you are curious, node relay traffic for clients for free to earn
  mining rewards in NKN blockchain), while Tuna mode requires listener to pay
  NKN token directly to Tuna service providers.

A diagram of the default NKN client mode:

```
                                 A - ... - X
                               /             \
Alice <--> TCP <--> NKN client - B - ... - Y - NKN client <--> TCP <--> Bob
                               \             /
                                 C - ... - Z
```

A diagram of the Tuna mode:

```
                                      A
                                    /   \
Alice <--> TCP <--> NKN Tuna client - B - NKN Tuna client <--> TCP <--> Bob
                                    \   /
                                      C
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

## Tuna Mode

Add `-tuna` on both side of the tunnel to use Tuna mode, which has much better
performance but requires listener to pay NKN token directly to Tuna service
providers.

## Contributing

**Can I submit a bug, suggestion or feature request?**

Yes. Please open an issue for that.

**Can I contribute patches?**

Yes, we appreciate your help! To make contributions, please fork the repo, push
your changes to the forked repo with signed-off commits, and open a pull request
here.

Please sign off your commit. This means adding a line "Signed-off-by: Name
<email>" at the end of each commit, indicating that you wrote the code and have
the right to pass it on as an open source patch. This can be done automatically
by adding -s when committing:

```shell
git commit -s
```

## Community

* [Discord](https://discord.gg/c7mTynX)
* [Telegram](https://t.me/nknorg)
* [Reddit](https://www.reddit.com/r/nknblockchain/)
* [Twitter](https://twitter.com/NKN_ORG)
