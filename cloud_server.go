package expose

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

// NewServer - creates a http server to handle incomming requests
func NewCloudServer() *cloudServer {
	s := &cloudServer{}
	s.edgeConnections = make(map[string]*multiplexer)
	return s
}

// a simple http handler that receives a request and proxies it to a tcp connection
type cloudServer struct {
	edgeConnections  map[string]*multiplexer
	lock             sync.RWMutex
	currentSessionID int
}

func (s *cloudServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// http proxy to tcp
	// 1. get the connection
	// 2. write the request
	// 3. read the response
	// 4. write the response

	hostName := strings.Split(strings.Split(r.Host, ":")[0], ".")[0]

	s.lock.RLock()
	conn, ok := s.edgeConnections[hostName]
	s.lock.RUnlock()
	if !ok {
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprintln(w, "no connection found for host", hostName)
		return
	}
	pr, err := http.NewRequest(r.Method, r.URL.String(), r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprintln(w, "error creating proxy request", err)
		return
	}
	pr.Header = r.Header
	if r.Header.Get("Upgrade") == "websocket" {
		conn.UpgradeToWebsocket(w, pr)
		return
	}
	resp, err := conn.Do(pr)
	log.Println("cloudy: got response")
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprintln(w, "error writing proxy request", err)
		return
	}
	defer resp.Body.Close()

	w.WriteHeader(resp.StatusCode)
	for k, v := range resp.Header {
		w.Header().Set(k, v[0])
	}
	log.Println("cloudy: writing response body")
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		fmt.Fprintln(w, "error writing proxy response", err)
		return
	}
}

func generateNewHostname() string {
	return "host" + strconv.Itoa(rand.Intn(100000))
}

// AcceptEdgeConnection - accepts a connection and creates a new http client
func (s *cloudServer) AcceptEdgeConnection(conn io.ReadWriter) error {
	host, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		log.Println("error reading from connection", err)
		return err
	}

	host = strings.Trim(host, "\n")
	s.lock.Lock()
	defer s.lock.Unlock()
	if host == NoExplicitHostRequest {
		// generate a new host
		for {
			host = generateNewHostname()
			if _, ok := s.edgeConnections[host]; !ok {
				break
			}
		}
	}

	if _, ok := s.edgeConnections[host]; ok {
		conn.Write([]byte("ERR connection already exists\r\n"))
		return nil
	}
	_, err = conn.Write([]byte("OK " + host + "\r\n"))
	if err != nil {
		log.Println("error writing to connection", err)
		return err
	}

	s.edgeConnections[host] = NewMultiplexer(conn)
	go s.edgeConnections[host].ReadLoop()

	return nil
}
