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
	mappingAddr := flag.String("m", "", "STUN local addr used for mapping behavior discovery. ip or ip:port")
	filteringAddr := flag.String("f", "", "STUN local addr used for filtering behavior discovery. ip or ip:port")
	verbose := flag.Bool("v", false, "Verbose")

	flag.Parse()
	if !strings.Contains(*mappingAddr, ":") {
		*mappingAddr = *mappingAddr + ":0"
	}
	if !strings.Contains(*filteringAddr, ":") {
		*filteringAddr = *filteringAddr + ":0"
	}
	n, err := nats.NewNATS(&nats.Config{
		Server:         *server + ":" + *port,
		Verbose:        *verbose,
		MappingLocal:   *mappingAddr,
		FilteringLocal: *filteringAddr,
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
