package main

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
)

// Config stores server configuration parameters
type Config struct {
	BufSize          int    `json:"bufSize"`          // buffer size
	StompURI         string `json:"stompURI"`         // StompAMQ URI
	StompLogin       string `json:"stompLogin"`       // StompAQM login name
	StompPassword    string `json:"stompPassword"`    // StompAQM password
	StompIterations  int    `json:"stompIterations"`  // Stomp iterations
	StompSendTimeout int    `json:"stompSendTimeout"` // heartbeat send timeout
	StompRecvTimeout int    `json:"stompRecvTimeout"` // heartbeat recv timeout
	Endpoint         string `json:"endpoint"`         // StompAMQ endpoint
	ContentType      string `json:"contentType"`      // ContentType of UDP packet
	Verbose          int    `json:"verbose"`          // verbosity level
}

// helper function to resolve Stomp URI into list of addr:port pairs
func resolveURI(uri string) ([]string, error) {
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
			out = append(out, fmt.Sprintf("%s:%s", addr, port))
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
func (s *StompManager) resetConnection() {
	log.Println("reset all connections to StompAMQ", s.Config.StompURI)
	for _, c := range s.ConnectionPool {
		if c != nil {
			c.Disconnect()
		}
		c = nil
	}
}

// get new stomp connection
func (s *StompManager) getConnection() (*stomp.Conn, string, error) {
	if len(s.ConnectionPool) > 0 && len(s.ConnectionPool) == len(s.Addresses) {
		idx := rand.Intn(len(s.ConnectionPool))
		addr := s.Addresses[idx]
		conn := s.ConnectionPool[idx]
		if conn != nil {
			return conn, addr, nil
		}
	}
	if s.Config.StompURI == "" {
		err := errors.New("Unable to connect to Stomp, not URI")
		return nil, "", err
	}
	if s.Config.StompLogin == "" {
		err := errors.New("Unable to connect to Stomp, not login")
		return nil, "", err
	}
	if s.Config.StompPassword == "" {
		err := errors.New("Unable to connect to Stomp, not password")
		return nil, "", err
	}
	if len(s.Addresses) == 0 {
		addrs, err := resolveURI(s.Config.StompURI)
		if err != nil {
			err := errors.New(fmt.Sprintf("Unable to resolve StompURI, error %v", err))
			return nil, "", err
		}
		s.Addresses = addrs
	}
	// in case of test login return
	if s.Config.StompLogin == "test" {
		idx := rand.Intn(len(s.Addresses))
		addr := s.Addresses[idx]
		return nil, addr, nil
	}
	// make connection pool equal to number of IP addresses we have
	s.ConnectionPool = make([]*stomp.Conn, len(s.Addresses))
	sendTimeout := time.Duration(s.Config.StompSendTimeout)
	recvTimeout := time.Duration(s.Config.StompRecvTimeout)
	for idx, addr := range s.Addresses {
		conn, err := stomp.Dial("tcp", addr,
			stomp.ConnOpt.Login(s.Config.StompLogin, s.Config.StompPassword),
			stomp.ConnOpt.HeartBeat(sendTimeout*time.Millisecond, recvTimeout*time.Millisecond),
		)
		if err != nil {
			log.Printf("Unable to connect to %s, error %v\n", addr, err)
		} else {
			log.Printf("connected to StompAMQ server %s\n", addr)
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
func (s *StompManager) Send(data []byte) error {
	var err error
	conn, addr, err := s.getConnection()
	for i := 0; i < s.Config.StompIterations; i++ {
		// we send data using existing stomp connection
		err = conn.Send(s.Config.Endpoint, s.Config.ContentType, data)
		if err == nil {
			if s.Config.Verbose > 0 {
				log.Printf("send data to %s endpoint %s\n", addr, s.Config.Endpoint)
			}
			return nil
		}
		// since we fail we'll acquire new stomp connection and retry
		if i == s.Config.StompIterations-1 {
			log.Printf("unable to send data to %s, error %v, iteration %d\n", s.Config.Endpoint, err, i)
		} else {
			log.Printf("unable to send data to %s, error %v, iteration %d\n", s.Config.Endpoint, err, i)
		}
		s.resetConnection()
		conn, addr, err = s.getConnection()
		if err != nil {
			log.Printf("Unable to get connection, %v\n", err)
		}
	}
	return err
}

// New creates new instance of StompManager
func New(config Config) *StompManager {
	rand.Seed(12345)
	mgr := StompManager{Config: config}
	return &mgr
}