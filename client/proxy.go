package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"gopkg.in/bufio.v1"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	stateInitial = iota
	stateConnecting
	stateKeepalive
	stateReceiving
	stateSending
	stateIdle
)

var stateTransformations = map[int]map[int]struct{}{
	stateInitial:    {stateConnecting: struct{}{}},
	stateConnecting: {stateIdle: struct{}{}},
	stateIdle:       {stateKeepalive: struct{}{}, stateReceiving: struct{}{}},
	stateKeepalive:  {stateIdle: struct{}{}},
	stateReceiving:  {stateSending: struct{}{}},
	stateSending:    {stateIdle: struct{}{}},
}

type proxy struct {
	state int
	sync.Mutex
	conn net.Conn
	name string
	pass string
	localHost string
	serverHost string
	ctx context.Context
	statCollector func(entry statEntry)
}

func newProxy(host string, name, pass string, sc func(entry statEntry)) *proxy {
	p := new(proxy)
	p.localHost = host
	p.name = name
	p.pass = pass
	p.statCollector = sc

	return p
}

func(p *proxy) reconnect() error{
	var err error

	for n :=0; n < 3; n++ {
		select {
		case<- p.ctx.Done():
			return p.ctx.Err()
			default:
			if err := p.connect(p.ctx, p.serverHost); err !=nil {
				log.Println("failed to connect, reconnecting")
				time.Sleep(3 * time.Second)
			} else {
				log.Println("reconnected")
				return nil
			}
		}
	}
	return err
}

func (p *proxy) connect(ctx context.Context, host string) error {
	p.serverHost = host
	p.ctx = ctx
	/**if !p.setState(stateConnecting) {
		log.Println("invalid state while connecting", p.state)
		return unknownError
	}

	 */
	log.Println("connecting")
	var err error
	var d net.Dialer
	p.conn, err =d.DialContext(ctx, "tcp4", host)
	if err != nil {
		log.Println("failed to connect", err)
		return err
	}

	fmt.Fprintf(p.conn, "%s %s\n", p.name, p.pass)

	_, err = fmt.Fscanf(p.conn, "%s %s\n", &p.name, &p.pass)
	if err != nil || p.pass == "" || p.name == "" {
		log.Println("failed to register", p.name, p.pass, "error:", err)
		return err
	}
	log.Println("should be registered, I have to reply with ready")
	fmt.Fprintln(p.conn, "ready")
/**
	if !p.setState(stateIdle) {
		log.Println("invalid state while connecting", p.state)
		return unknownError
	}

 */
	return nil
}

func (p *proxy) setState(state int) bool {
	p.Lock()
	defer p.Unlock()

	if _, ok := stateTransformations[p.state][state]; ok {
		p.state = state
		return true
	}
	log.Println("cannot set state ", state, "from state", p.state)
	return false
}

func (p *proxy) loop(ctx context.Context, wg *sync.WaitGroup) error {
	defer wg.Done()
	cli := http.DefaultClient
	cli.Timeout = 3 * time.Second
	log.Println("looping")

	for {
		select {
		case <- ctx.Done():
			log.Println("exiting loop")
			return nil
		default:
			err := p.read(cli)
			if err != nil {
				log.Println("what to do with the err", err)
				if err := p.reconnect() ; err != nil{
					return err
				}
			}
		}
	}
}


func (p *proxy) read(cli *http.Client) error {
	stat := statEntry{
		Took:      0,
	}
	var method, url string
	var hs, bs int

	n, err := fmt.Fscanf(p.conn, "%s %s %d %d", &method, &url, &hs, &bs)
	if err != nil {
		log.Println("error reading", err)
		return err
	}
	log.Println("received n bytes from fscanf", n)
	stat.Timestamp = time.Now()
	if method == "" || url == "" {
		log.Println("invalid request received")
		return unknownError
	}
	url = p.localHost + url
	hb := make([]byte, hs)
	bb := make([]byte, bs)

	hn, err := io.ReadFull(p.conn, hb)
	if err != nil || hn != hs {
		log.Println("invalid header", hn, hs, string(hb))
	}
	bn, err := io.ReadFull(p.conn, bb)
	if err != nil || bn != bs {
		log.Println("invalid body", bn, bs, string(bb))
		return unknownError
	}
	br := bufio.NewBuffer(bb)
	req, err := http.NewRequest(method, url, br)

	if err != nil {
		log.Println("failed to create request", err)
		return unknownError
	}
	headerString := string(hb)
	for _, l := range strings.Split(headerString, "\n") {
		var k, v string
		fmt.Sscanf(l, "%s %s", &k, &v)
		k = strings.Trim(k, ":")
		if len(k)>0 {
			req.Header.Add(k, v)
		}
	}
	res, err := cli.Do(req)
	if err != nil {
		log.Println("failed to exec request", err)
		return unknownError
	}

	headerBuffer := bytes.NewBuffer([]byte{})
	res.Header.Write(headerBuffer)

	fmt.Fprintf(p.conn, "%d %d %d\n", res.StatusCode, headerBuffer.Len(), res.ContentLength)
	io.Copy(p.conn, headerBuffer)
	io.Copy(p.conn, res.Body)
	stat.Request = *req
	stat.Response = *res
	stat.Took = time.Now().Sub(stat.Timestamp)
	p.statCollector(stat)
	return nil
}

var unknownError = errors.New("something happened")
