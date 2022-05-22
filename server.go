package main

import (
	"flag"
	"fmt"
	"github.com/enobufs/go-nats/nats"
	"net"
	"sync"

	"github.com/pion/logging"
	"github.com/pion/stun"
	"github.com/pion/transport/vnet"
)

const (
	priToSecUri string = "/v1/gostun/pri2sec"
)

type STUNServerConfig struct {
	PrimaryAddress   string
	SecondaryAddress string
	Net              *vnet.Net
	role             string
	pri2SecHost      string
	//LoggerFactory    logging.LoggerFactory
}
type priToSec struct {
	From  *net.UDPAddr  `json:"from"`
	M     *stun.Message `json:"m"`
	Index int           `json:"index"`
}

type STUNServer struct {
	priAddrs    []*net.UDPAddr
	secAddrs    []*net.UDPAddr
	conns       []net.PacketConn
	software    stun.Software
	net         *vnet.Net
	log         logging.LeveledLogger
	pri2SecHost string
	role        string
}

func main() {
	primaryAddr := flag.String("p", "127.0.0.1:3478", "STUN primary server address.")
	secondaryAddress := flag.String("s", "192.168.31.24:3479", "STUN secondary server addr")
	pri2SecHost := flag.String("p2s", "", "STUN primary server to secondary server")
	// role both,说明2个ip都在同一个机器上，否则是分开部署
	role := flag.String("r", "both", "this server role")

	flag.Parse()
	if *role != "both" && *role != "pri" && *role != "sec" {
		fmt.Println("wrong role, only support: both 、 pri 、 sec")
		return
	}
	if *role == "pri" && *pri2SecHost == "" {
		fmt.Println("need pri2SecHost when current role is pri")
		return
	}

	s, err := nats.NewSTUNServer(&nats.STUNServerConfig{PrimaryAddress: *primaryAddr, SecondaryAddress: *secondaryAddress, Role: *role, Pri2SecHost: *pri2SecHost})
	if err != nil {
		fmt.Println("err new stun server")
		return
	}
	wg := sync.WaitGroup{}
	wg.Add(1)
	s.Start()
	if *role == "sec" {
		s.StartListenServer()
	}
	wg.Wait()

}
