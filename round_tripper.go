package expose

import (
	"bufio"
	"io"
	"log"

	// "net"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"
	"sync"
)

type proxyConn struct {
	conn  io.ReadWriter // net.Conn
	mutex sync.Mutex
}

func NewProxyConn(conn io.ReadWriter) *proxyConn {
	p := &proxyConn{}
	p.conn = conn
	return p
}

func (p *proxyConn) RoundTrip(req *http.Request) (*http.Response, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	err := req.Write(p.conn)
	if err != nil {
		return nil, err
	}

	resp := &http.Response{}
	respReader := textproto.NewReader(bufio.NewReader(p.conn))
	body := ""

	firstLine, err := respReader.ReadLine()
	if err != nil {
		log.Println("rt: error reading from connection", err)
		return nil, err
	}
	parts := strings.Split(firstLine, " ")
	if len(parts) < 3 {
		log.Println("rt: invalid first line", firstLine)
		return nil, err
	}
	resp.StatusCode, _ = strconv.Atoi(parts[1])
	header, err := respReader.ReadMIMEHeader()
	if err != nil {
		log.Println("rt: error reading header from connection", err)
		return nil, err
	}
	resp.Header = make(http.Header)

	for k, v := range header {
		if len(v) > 0 {
			resp.Header.Set(k, v[0])
		}
	}

	for {

		line, err := respReader.ReadLine()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Println("rt: error reading lines from connection", err)
		}
		if line == EndOfResponseMarker {
			break
		}
		body += line
	}

	if body != "" {
		resp.Body = io.NopCloser(strings.NewReader(body))
	}

	return resp, nil
}
