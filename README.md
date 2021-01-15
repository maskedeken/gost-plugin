# Yet another SIP003 plugin for shadowsocks, based on GOST Tunnel

## Build

* `go build`

## Usage

See command line args for advanced usages.

### Shadowsocks over TLS/Multiplex TLS 

On your server

```sh
ss-server -c config.json -p 443 --plugin gost-plugin --plugin-opts "server;mode=tls"
ss-server -c config.json -p 443 --plugin gost-plugin --plugin-opts "server;mode=mtls"
```

On your client

```sh
ss-local -c config.json -p 443 --plugin gost-plugin --plugin-opts "mode=tls"
ss-local -c config.json -p 443 --plugin gost-plugin --plugin-opts "mode=mtls;mux=1"
```

### Shadowsocks over Websocket/Multiplex Websocket (HTTP)

On your server

```sh
ss-server -c config.json -p 80 --plugin gost-plugin --plugin-opts "server;mode=ws"
ss-server -c config.json -p 80 --plugin gost-plugin --plugin-opts "server;mode=mws"
```

On your client

```sh
ss-local -c config.json -p 80 --plugin gost-plugin --plugin-opts "mode=ws"
ss-local -c config.json -p 80 --plugin gost-plugin --plugin-opts "mode=mws;mux=1"
```

### Shadowsocks over Websocket/Multiplex Websocket (HTTPS)

On your server

```sh
ss-server -c config.json -p 443 --plugin gost-plugin --plugin-opts "server;cert=cert.pem;key=key.pem;mode=wss"
ss-server -c config.json -p 443 --plugin gost-plugin --plugin-opts "server;cert=cert.pem;key=key.pem;mode=mwss"
```

On your client

```sh
ss-local -c config.json -p 443 --plugin gost-plugin --plugin-opts "host=mydomain.me;mode=wss"
ss-local -c config.json -p 443 --plugin gost-plugin --plugin-opts "host=mydomain.me;mode=mwss;mux=1"
```

