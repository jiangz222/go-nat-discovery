package nats

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/pion/logging"
	"github.com/pion/stun"
	"github.com/pion/transport/vnet"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strings"
)

const (
	priToSecUri string = "/v1/gostun/pri2sec"
)

type STUNServerConfig struct {
	PrimaryAddress   string
	SecondaryAddress string
	Net              *vnet.Net
	Role             string
	Pri2SecHost      string
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

// parseReq 解析http请求
func parseReq(req *http.Request, result interface{}) error {
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		fmt.Println("Error reading body: ", err)
		return errors.New("can't read body")
	}
	if err = json.Unmarshal(body, result); err != nil {
		fmt.Println("Error unmarshal body: ", err)
		return errors.New("can't read body")
	}
	return nil
}

func (s *STUNServer) priToSecHandler(w http.ResponseWriter, r *http.Request) {
	pts := &priToSec{}
	err := parseReq(r, pts)
	if err != nil {
		s.log.Errorf("parseReq err from pri %v", err)
		http.Error(w, "parseReq err from pri ", http.StatusBadRequest)
		return
	}
	err = s.handleBindingRequest(pts.From, pts.M, s.conns[pts.Index-2])
	if err != nil {
		s.log.Errorf("handleBindingRequest err")
		http.Error(w, "handleBindingRequest err", http.StatusInternalServerError)
		return
	}
}
func (s *STUNServer) StartListenServer() {
	http.HandleFunc(priToSecUri, s.priToSecHandler)
	http.ListenAndServe(s.pri2SecHost, nil)
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

	return &STUNServer{priAddrs: priAddrs, secAddrs: secAddrs, net: config.Net, log: log, role: config.Role, pri2SecHost: config.Pri2SecHost}, nil
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
		conn, err := s.getConn(index, from, m)
		if err != nil || conn == nil {
			s.log.Warnf("get connection failure %v, or conn to sec", err)
			continue
		}
		err = s.handleBindingRequest(from, m, conn)
		if err != nil {
			s.log.Errorf("readLoop: handleBindingRequest failed: %s", err.Error())
			return
		}
	}
}
func (s *STUNServer) getConn(index int, from net.Addr, m *stun.Message) (conn net.PacketConn, err error) {
	// Check CHANGE-REQUEST
	changeReq := attrChangeRequest{}
	err = changeReq.GetFrom(m)
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
		if index >= len(s.conns) {
			if s.role == "pri" {
				return nil, s.sendMsgToSec(index, from, m)
			} else {
				s.log.Errorf("not expect %d %s", index, s.role)
				return nil, errors.New("not expect")
			}
		} else {
			conn = s.conns[index]
		}
	}
	return conn, nil
}

func (s *STUNServer) handleBindingRequest(from net.Addr, m *stun.Message, conn net.PacketConn) error {
	s.log.Debugf("received BindingRequest from %s", from.String())

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

	//s.log.Infof("%+v %+v %+v", conn, msg, from)
	_, err = conn.WriteTo(msg.Raw, from)
	if err != nil {
		return err
	}
	return nil
}
func (s *STUNServer) sendMsgToSec(index int, from net.Addr, m *stun.Message) error {
	client := http.Client{}
	fromUDP := from.(*net.UDPAddr)
	pts := priToSec{
		From:  fromUDP,
		M:     m,
		Index: index,
	}
	bytesPts, err := json.Marshal(pts)
	if err != nil {
		s.log.Warnf("marshal pts err: %s", err.Error())
		return err
	}
	req, err := http.NewRequest(http.MethodPost, "http://"+s.pri2SecHost+priToSecUri, bytes.NewReader(bytesPts))
	if err != nil {
		s.log.Warnf("NewRequest  err: %s", err.Error())
		return err
	}
	_, err = client.Do(req)
	if err != nil {
		s.log.Warnf("client do  err: %s", err.Error())
		return err
	}
	s.log.Debug("client do  success ")
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
