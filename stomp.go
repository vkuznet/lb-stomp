package lbstomp

// lb-stomp - Load-balancer access to stomp
//
// Copyright (c) 2020 - Valentin Kuznetsov <vkuznet@gmail.com>
//

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net"
	"strings"
	"time"

	"github.com/go-stomp/stomp"
	"github.com/go-stomp/stomp/frame"
)

// Config stores server configuration parameters
type Config struct {
	URI                  string  `json:"uri"`                  // Stomp AMQ URI
	Login                string  `json:"login"`                // Stomp AQM login name
	Password             string  `json:"password"`             // Stomp AQM password
	Iterations           int     `json:"iterations"`           // Stomp iterations
	SendTimeout          int     `json:"sendTimeout"`          // heartbeat send timeout
	RecvTimeout          int     `json:"recvTimeout"`          // heartbeat recv timeout
	HeartBeatGracePeriod float64 `json:"heartBeatGracePeriod"` // is used to calculate the read heart-beat timeout
	Endpoint             string  `json:"endpoint"`             // StompAMQ endpoint
	ContentType          string  `json:"contentType"`          // ContentType of UDP packet
	Protocol             string  `json:"protocol"`             // network protocol to use
	Verbose              int     `json:"verbose"`              // verbosity level
}

// helper function to resolve Stomp URI into list of addr:port pairs
func resolveURI(uri, protocol string) ([]string, error) {
	var out []string
	arr := strings.Split(uri, ":")
	host := arr[0]
	port := arr[1]
	addrs, err := net.LookupIP(host)
	if err != nil {
		log.Printf("Unable to resolve host %s into IP addresses, error %v\n", host, err)
		return out, err
	}
	for _, addr := range addrs {
		// use only IPv4 addresses
		if strings.Contains(addr.String(), ".") {
			if protocol == "tcp" || protocol == "tcp4" {
				out = append(out, fmt.Sprintf("%s:%s", addr, port))
			}
		} else if strings.Contains(addr.String(), ":") {
			// for ipv6 network address we should use [ipv6]:port notations
			// see https://golang.org/pkg/net/
			if protocol == "tcp" || protocol == "tcp6" {
				out = append(out, fmt.Sprintf("[%s]:%s", addr, port))
			}
		}
	}
	return out, nil
}

// StompManager hanles connection to Stomp AMQ Broker
type StompManager struct {
	Addresses      []string      // stomp addresses
	ConnectionPool []*stomp.Conn // pool of connections to stomp AMQ Broker
	Config         Config        // stomp configuration
}

// reset all stomp connections
func (s *StompManager) ResetConnection() {
	log.Println("reset all connections to StompAMQ", s.Config.URI)
	for _, c := range s.ConnectionPool {
		if c != nil {
			c.Disconnect()
		}
		c = nil
	}
	// reset connection pool
	s.ConnectionPool = nil
}

// get new stomp connection
func (s *StompManager) GetConnection() (*stomp.Conn, string, error) {
	if len(s.ConnectionPool) > 0 && len(s.ConnectionPool) == len(s.Addresses) {
		idx := rand.Intn(len(s.ConnectionPool))
		addr := s.Addresses[idx]
		conn := s.ConnectionPool[idx]
		if conn != nil {
			return conn, addr, nil
		}
	}
	if s.Config.URI == "" {
		err := errors.New("Unable to connect to Stomp, not URI")
		return nil, "", err
	}
	if s.Config.Login == "" {
		err := errors.New("Unable to connect to Stomp, not login")
		return nil, "", err
	}
	if s.Config.Password == "" {
		err := errors.New("Unable to connect to Stomp, not password")
		return nil, "", err
	}
	if s.Config.Protocol == "" {
		s.Config.Protocol = "tcp4"
	}
	if len(s.Addresses) == 0 {
		addrs, err := resolveURI(s.Config.URI, s.Config.Protocol)
		if s.Config.Verbose > 0 {
			for _, addr := range addrs {
				log.Println("use", addr)
			}
		}
		if err != nil {
			err := errors.New(fmt.Sprintf("Unable to resolve URI, error %v", err))
			return nil, "", err
		}
		s.Addresses = addrs
	}
	if len(s.Addresses) == 0 {
		return nil, "", errors.New("no valid IP addresses found")
	}
	// in case of test login return
	if s.Config.Login == "test" {
		idx := rand.Intn(len(s.Addresses))
		addr := s.Addresses[idx]
		return nil, addr, nil
	}
	// make connection pool equal to number of IP addresses we have
	s.ConnectionPool = make([]*stomp.Conn, len(s.Addresses))
	sendTimeout := time.Duration(s.Config.SendTimeout)
	recvTimeout := time.Duration(s.Config.RecvTimeout)
	heartBeatGracePeriod := s.Config.HeartBeatGracePeriod
	if heartBeatGracePeriod == 0 {
		heartBeatGracePeriod = 1
	}
	for idx, addr := range s.Addresses {
		conn, err := stomp.Dial(s.Config.Protocol, addr,
			stomp.ConnOpt.Login(s.Config.Login, s.Config.Password),
			stomp.ConnOpt.HeartBeat(sendTimeout*time.Millisecond, recvTimeout*time.Millisecond),
			stomp.ConnOpt.HeartBeatGracePeriodMultiplier(heartBeatGracePeriod),
		)
		if err != nil {
			log.Fatalf("Unable to connect to '%s', error %v\n", addr, err)
		} else {
			log.Printf("connected to StompAMQ server '%s'\n", addr)
		}
		s.ConnectionPool[idx] = conn
	}
	// pick-up random connection
	idx := rand.Intn(len(s.ConnectionPool))
	conn := s.ConnectionPool[idx]
	addr := s.Addresses[idx]
	return conn, addr, nil
}

// helper function to send data to stomp
func (s *StompManager) Send(data []byte, opts ...func(*frame.Frame) error) error {
	var err error
	conn, addr, err := s.GetConnection()
	for i := 0; i < s.Config.Iterations; i++ {
		// we send data using existing stomp connection
		err = conn.Send(s.Config.Endpoint, s.Config.ContentType, data, opts...)
		if err == nil {
			if s.Config.Verbose > 0 {
				log.Printf("send data to %s endpoint %s\n", addr, s.Config.Endpoint)
			}
			return nil
		}
		log.Println("fail to send data", err)
		// since we fail we'll acquire new stomp connection and retry
		if i == s.Config.Iterations-1 {
			log.Printf("unable to send data to %s, error %v, iteration %d\n", s.Config.Endpoint, err, i)
		} else {
			log.Printf("unable to send data to %s, error %v, iteration %d\n", s.Config.Endpoint, err, i)
		}
		s.ResetConnection()
		conn, addr, err = s.GetConnection()
		if err != nil {
			log.Printf("Unable to get connection, %v\n", err)
		}
	}
	return err
}

// String represents Stomp Manager
func (s *StompManager) String() string {
	r := fmt.Sprintf("<StompManager: addrs=%+v, endpoint=%s, iters=%v, sendTimeout=%v, recvTimeout=%v, protocol=%s, verbose=%v>", s.Addresses, s.Config.Endpoint, s.Config.Iterations, s.Config.SendTimeout, s.Config.RecvTimeout, s.Config.Protocol, s.Config.Verbose)
	return r
}

// New creates new instance of StompManager
func New(config Config) *StompManager {
	rand.Seed(12345)
	mgr := StompManager{Config: config}
	// initialize connections
	mgr.GetConnection()
	return &mgr
}
