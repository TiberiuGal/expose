package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"gopkg.in/bufio.v1"
)

var (
	errUnknownError = errors.New("some error")
)

type proxy struct {
	state int
	sync.Mutex
	conn          net.Conn
	name          string
	pass          string
	localHost     string
	serverHost    string
	ctx           context.Context
	statCollector func(entry statEntry)
}

func newProxy(server, local string, name, pass string, sc func(entry statEntry)) *proxy {
	p := proxy{
		localHost:     local,
		serverHost:    server,
		name:          name,
		pass:          pass,
		statCollector: sc,
	}
	return &p
}

func (p *proxy) reconnect(ctx context.Context) error {
	var err error

	for n := 0; n < 3; n++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := p.connect(ctx); err != nil {
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

func (p *proxy) connect(ctx context.Context) error {
	log.Println("connecting to server")
	var err error
	var d net.Dialer
	p.conn, err = d.DialContext(ctx, "tcp4", p.serverHost)
	if err != nil {
		return errors.Wrap(err, "failed wile connecting")
	}

	fmt.Fprintf(p.conn, "%s %s\n", p.name, p.pass)

	_, err = fmt.Fscanf(p.conn, "%s %s\n", &p.name, &p.pass)
	if err != nil {
		return errors.Wrap(err, "failed to register")
	}
	if p.pass == "" || p.name == "" {
		return fmt.Errorf("failed to register %s, %s", p.name, p.pass)
	}

	fmt.Fprintln(p.conn, "ready")
	log.Println("connected to server", p.serverHost, p.localHost, p.name, p.pass)
	return nil
}

func (p *proxy) loop(ctx context.Context, wg *sync.WaitGroup) error {
	defer wg.Done()
	cli := http.DefaultClient
	cli.Timeout = 3 * time.Second

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			err := p.read(cli)
			if err != nil {
				log.Println("what to do with the err", err)
				if err := p.reconnect(ctx); err != nil {
					return err
				}
			}
		}
	}
}

func (p *proxy) read(cli *http.Client) error {
	stat := statEntry{
		Took: 0,
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
		return errUnknownError
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
		return errUnknownError
	}
	br := bufio.NewBuffer(bb)
	req, err := http.NewRequest(method, url, br)

	if err != nil {
		log.Println("failed to create request", err)
		return errUnknownError
	}
	headerString := string(hb)
	for _, l := range strings.Split(headerString, "\n") {
		var k, v string
		fmt.Sscanf(l, "%s %s", &k, &v)
		k = strings.Trim(k, ":")
		if len(k) > 0 {
			req.Header.Add(k, v)
		}
	}
	res, err := cli.Do(req)
	if err != nil {
		log.Println("failed to exec request", err)
		return errUnknownError
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
