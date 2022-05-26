# go-nat-discovery

NAT type discovery tool using STUN written purely in Go, include client and server.

Rewrite the [go-nats](https://github.com/enobufs/go-nats) which is powered by [pion](https://pion.ly).

Add features:
- fix bugs in original go-nats' discover rules
- add stun discovery server 
- add slave and master server, for servers don't have two public ip

## Usage

### client 

```
# go run client.go -s stun.sipgate.net:3478 -i localIp:localPort 
```

should has output:
```
NAT Type: Blocked
External IP: None
External Port: None
```

### server

#### server has two public ip

```
# go run server.go -p publicIp-1:port-1 -s publicIP-2:port-2
```

#### server has one public ip

If you don't have two public ip on one server, then You must have two server, each one has one public ip, and primary server can communicate to secondary server via `primary2SecondaryHost:port`.

- server A, run as primary
```
# go run server.go -r pri -p publicIpOnPrimary:portA -s publicIpOnServerB:portB -p2s primary2SecondaryHost:port
```

- server B, run as secondary
```
# go run server.go -r sec -p publicIpOnPrimary:portA -s publicIpOnServerB:portB -p2s primary2SecondaryHost:port
```

