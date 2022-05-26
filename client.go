package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/jiangz222/go-nat-discovery/nats"
)

func check(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
		os.Exit(1)
	}
}

func main() {
	server := flag.String("H", "stun.sipgate.net", "STUN server address.")
	port := flag.String("P", "3478", "STUN server port.")
	localAddr := flag.String("i", "", "STUN local addr. ip or ip:port")
	verbose := flag.Bool("v", false, "Verbose")

	flag.Parse()
	if !strings.Contains(*localAddr, ":") {
		*localAddr = *localAddr + ":0"
	}
	n, err := nats.NewNATS(&nats.Config{
		Server:  *server + ":" + *port,
		Verbose: *verbose,
		Local:   *localAddr,
	})
	check(err)

	res, err := n.Discover()
	check(err)

	_, err = json.MarshalIndent(res, "", "  ")
	check(err)

	// change output as https://github.com/jtriley/pystun
	fmt.Printf("NAT Type: %s\nExternal IP: %s\nExternal Port: %s\n", res.NATType, res.ExternalIP, res.ExternalPort)
	//fmt.Println(string(bytes))
}
