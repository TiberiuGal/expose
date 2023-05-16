package expose

import (
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
)

// NewServer - creates a http server to handle incomming requests
func NewCloudServer() *cloudServer {
	s := &cloudServer{}
	s.connections = make(map[string]*http.Client)
	return s
}

// a simple http handler that receives a request and proxies it to a tcp connection
type cloudServer struct {
	connections map[string]*http.Client
}

func (s *cloudServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// http proxy to tcp
	// 1. get the connection
	// 2. write the request
	// 3. read the response
	// 4. write the response

	hostName := strings.Split(strings.Split(r.Host, ":")[0], ".")[0]

	conn, ok := s.connections[hostName]
	if !ok {
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprintln(w, "no connection found for host", hostName)
		log.Printf("no connection found for host %s, %+v \n", hostName, s.connections)
		return
	}
	pr, err := http.NewRequest(r.Method, r.URL.String(), r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)

		fmt.Fprintln(w, "error creating proxy request", err)
		return
	}
	pr.Header = r.Header
	log.Println("proxying request to", hostName)
	resp, err := conn.Do(pr)
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprintln(w, "error writing proxy request", err)
		return
	}
	defer resp.Body.Close()
	log.Println("got response", resp.Status)
	w.WriteHeader(resp.StatusCode)
	for k, v := range resp.Header {
		w.Header().Set(k, v[0])
		log.Println("header from response", k, v[0])
	}

	// w.Write([]byte("abc\r\n"))
	respBytes, err := io.ReadAll(resp.Body)
	log.Println("read response body", len(respBytes), respBytes)
	n, err := w.Write(respBytes)
	// n, err := bufio.NewReader(resp.Body).WriteTo(w) //
	// n, err := io.Copy(w, resp.Body)
	if err != nil {
		fmt.Fprintln(w, "error writing proxy response", err)
		return
	}
	// w.Write(respBytes)
	log.Println("wrote response", resp.Status, len(respBytes), n)
}

func generateNewHostname() string {
	return "host" + strconv.Itoa(rand.Intn(100000))
}

func (s *cloudServer) Accept(conn io.ReadWriter) error {
	buff := make([]byte, 1024)
	n, err := conn.Read(buff)
	if err != nil {
		log.Println("error reading from connection", err)
		return err
	}
	log.Println("read", n, "bytes from connection", buff[:n])
	host := string(buff[:n-2])
	if host == NoExplicitHostRequest {
		for {
			host = generateNewHostname()
			if _, ok := s.connections[host]; !ok {
				break
			}
		}
	}

	if _, ok := s.connections[host]; ok {
		log.Println("connection already exists for host", host)
		conn.Write([]byte("ERR connection already exists\r\n"))
		return nil
	}
	conn.Write([]byte("OK " + host + "\r\n"))
	client := http.Client{
		Transport: NewProxyConn(conn),
	}
	s.connections[host] = &client
	log.Println("new connection accepted for host", host)
	return nil
}
