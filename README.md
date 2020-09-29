### lb-stomp

[![Build Status](https://travis-ci.org/vkuznet/lb-stomp.svg?branch=master)](https://travis-ci.org/vkuznet/lb-stomp)
[![Go Report Card](https://goreportcard.com/badge/github.com/vkuznet/lb-stomp)](https://goreportcard.com/report/github.com/vkuznet/lb-stomp)
[![GoDoc](https://godoc.org/github.com/vkuznet/lb-stomp?status.svg)](https://godoc.org/github.com/vkuznet/lb-stomp)

Load-balanced Stomp (lb-stomp) package provides load-balanced
interface to Stomp AMQ brokers.

Built and test code:
```
go build
go test
```

Example how to use it
```
import stomp "github.com/vkuznet/lb-stomp"

// create new manager
config := stomp.Config{StompURI: "abc:123", StompLogin: "test", StompPassword: "test"}
mgr := stomp.New(config)
data := `{"test": 1}`
err := stomp.Send([]byte(data))
```
