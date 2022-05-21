package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"

	"github.com/pion/logging"
	"github.com/pion/stun"
	"github.com/pion/transport/vnet"
)

type STUNServerConfig struct {
	PrimaryAddress   string
	SecondaryAddress string
	Net              *vnet.Net
	role             string
	//LoggerFactory    logging.LoggerFactory
}

type STUNServer struct {
	priAddrs []*net.UDPAddr
	secAddrs []*net.UDPAddr
	conns    []net.PacketConn
	software stun.Software
	net      *vnet.Net
	log      logging.LeveledLogger
	role     string
}

func main() {
	primaryAddr := flag.String("p", "127.0.0.1:3478", "STUN primary server address.")
	secondaryAddress := flag.String("s", "192.168.31.24:3479", "STUN secondary server addr")
	// role both,说明2个ip都在同一个机器上，否则是分开部署
	role := flag.String("r", "both", "this server role")

	flag.Parse()
	if *role != "both" && *role != "pri" && *role != "sec" {
		fmt.Println("wrong role, only support: both 、 pri 、 sec")
		return
	}

	s, err := NewSTUNServer(&STUNServerConfig{PrimaryAddress: *primaryAddr, SecondaryAddress: *secondaryAddress, role: *role})
	if err != nil {
		fmt.Println("err new stun server")
		return
	}
	wg := sync.WaitGroup{}
	wg.Add(1)
	s.Start()
	wg.Wait()

}
func NewSTUNServer(config *STUNServerConfig) (*STUNServer, error) {
	//log := config.LoggerFactory.NewLogger("stun-serv")
	log := logging.NewDefaultLeveledLoggerForScope("", logging.LogLevelDebug, os.Stdout)

	pri := strings.Split(config.PrimaryAddress, ":")
	if len(pri) < 2 {
		pri = append(pri, "3478")
	}
	sec := strings.Split(config.SecondaryAddress, ":")
	if len(sec) < 2 {
		sec = append(sec, "3478")
	}

	if config.Net == nil {
		config.Net = vnet.NewNet(nil)
	}
	var priAddrs []*net.UDPAddr
	var secAddrs []*net.UDPAddr

	addr0, err := config.Net.ResolveUDPAddr(
		"udp", fmt.Sprintf("%s:%s", pri[0], pri[1]))
	if err != nil {
		return nil, err
	}
	priAddrs = append(priAddrs, addr0)

	addr1, err := config.Net.ResolveUDPAddr(
		"udp", fmt.Sprintf("%s:%s", pri[0], sec[1]))
	if err != nil {
		return nil, err
	}
	priAddrs = append(priAddrs, addr1)
	addr2, err := config.Net.ResolveUDPAddr(
		"udp", fmt.Sprintf("%s:%s", sec[0], pri[1]))
	if err != nil {
		return nil, err
	}
	secAddrs = append(secAddrs, addr2)

	addr3, err := config.Net.ResolveUDPAddr(
		"udp", fmt.Sprintf("%s:%s", sec[0], sec[1]))
	if err != nil {
		return nil, err
	}
	secAddrs = append(secAddrs, addr3)

	return &STUNServer{priAddrs: priAddrs, secAddrs: secAddrs, net: config.Net, log: log, role: config.role}, nil
}

func (s *STUNServer) Start() error {
	if s.role == "pri" || s.role == "both" {
		s.log.Warnf("%+v", s.priAddrs)
		for i, addr := range s.priAddrs {
			var err error
			s.log.Debugf("start listening on %s...", addr.String())
			conn, err := s.net.ListenUDP("udp", addr)
			s.conns = append(s.conns, conn)
			if err != nil {
				return err
			}
			go s.readLoop(i)
		}
	}
	if s.role == "sec" || s.role == "both" {
		s.log.Warnf("%+v", s.secAddrs)
		for i, addr := range s.secAddrs {
			var err error
			s.log.Debugf("start listening on %s...", addr.String())
			conn, err := s.net.ListenUDP("udp", addr)
			s.conns = append(s.conns, conn)
			if err != nil {
				return err
			}
			go s.readLoop(i)
		}
	}

	return nil
}

func (s *STUNServer) readLoop(index int) {
	conn := s.conns[index]
	for {
		buf := make([]byte, 1500)
		n, from, err := conn.ReadFrom(buf)
		if err != nil {
			s.log.Errorf("readLoop: %s", err.Error())
			return
		}

		s.log.Debugf("received %d bytes from %s", n, from.String())

		m := &stun.Message{Raw: append([]byte{}, buf[:n]...)}
		if err = m.Decode(); err != nil {
			s.log.Warnf("failed to decode: %s", err.Error())
			continue
		}

		if m.Type.Class != stun.ClassRequest {
			s.log.Warn("not a request. dropping...")
			continue
		}

		if m.Type.Method != stun.MethodBinding {
			s.log.Warn("not a binding request. dropping...")
			continue
		}

		err = s.handleBindingRequest(index, from, m)
		if err != nil {
			s.log.Errorf("readLoop: handleBindingRequest failed: %s", err.Error())
			return
		}
	}
}

func (s *STUNServer) handleBindingRequest(index int, from net.Addr, m *stun.Message) error {
	s.log.Debugf("received BindingRequest from %s", from.String())

	var conn net.PacketConn

	// Check CHANGE-REQUEST
	changeReq := attrChangeRequest{}
	err := changeReq.GetFrom(m)
	if err != nil {
		s.log.Debugf("CHANGE-REQUEST not found: %s", err.Error())
		conn = s.conns[index]
	} else {
		s.log.Debugf("CHANGE-REQUEST: changeIP=%v changePort=%v",
			changeReq.ChangeIP, changeReq.ChangePort)
		if changeReq.ChangeIP {
			index ^= 0x2
		}
		if changeReq.ChangePort {
			index ^= 0x1
		}
		conn = s.conns[index]
	}

	udpAddr := from.(*net.UDPAddr)

	attrs := s.makeAttrs(m.TransactionID, stun.BindingSuccess,
		&stun.XORMappedAddress{
			IP:   udpAddr.IP,
			Port: udpAddr.Port,
		},
		&stun.XORMappedAddress{
			IP:   udpAddr.IP,
			Port: udpAddr.Port,
		},
		&attrChangedAddress{
			attrAddress{
				IP:   s.priAddrs[1].IP,
				Port: s.priAddrs[1].Port,
			},
		},
		stun.Fingerprint)

	msg, err := stun.Build(attrs...)
	if err != nil {
		return err
	}

	s.log.Infof("%+v %+v %+v", conn, msg, from)
	_, err = conn.WriteTo(msg.Raw, from)
	if err != nil {
		return err
	}
	return nil
}

func (s *STUNServer) makeAttrs(
	transactionID [stun.TransactionIDSize]byte,
	msgType stun.MessageType,
	additional ...stun.Setter) []stun.Setter {
	attrs := append([]stun.Setter{&stun.Message{TransactionID: transactionID}, msgType}, additional...)
	if len(s.software) > 0 {
		attrs = append(attrs, s.software)
	}
	return attrs
}

func (s *STUNServer) Close() error {
	var err error
	for _, conn := range s.conns {
		if conn != nil {
			err2 := conn.Close()
			if err2 != nil && err == nil {
				err = err2
			}
		}
	}
	return err
}
