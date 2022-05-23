package main

import (
	"flag"
	"fmt"
	"github.com/enobufs/go-nats/nats"
	"sync"
)

func main() {
	primaryAddr := flag.String("p", "", "STUN primary server address.")
	secondaryAddr := flag.String("s", "", "STUN secondary server addr")
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
	if *primaryAddr == "" || *secondaryAddr == "" {
		fmt.Println("need primary addr and secondary addr")
		return
	}

	s, err := nats.NewSTUNServer(&nats.STUNServerConfig{PrimaryAddress: *primaryAddr, SecondaryAddress: *secondaryAddr, Role: *role, Pri2SecHost: *pri2SecHost})
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
