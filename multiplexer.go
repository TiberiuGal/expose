package expose

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"unicode/utf8"

	"github.com/gorilla/websocket"
)

type multiplexer struct {
	conn          io.ReadWriter
	streams       map[int]chan []byte
	lock          sync.RWMutex
	nextSessionId int32
}

func NewMultiplexer(conn io.ReadWriter) *multiplexer {
	return &multiplexer{conn: conn, streams: make(map[int]chan []byte)}
}

func (m *multiplexer) Open(id int) {
	m.streams[id] = make(chan []byte)
}

func (m *multiplexer) ReadLoop() {
	r := bufio.NewReader(m.conn)
	for {
		bb, err := r.ReadBytes('\n')
		if err == io.EOF {
			continue
		} else if err != nil {
			log.Println("error reading", err)
			continue
		}
		log.Println("got line", string(bb))
		streamId := decodeId(bb[:4])
		if string(bb[4:len(bb)-1]) == EndOfResponseMarker {
			log.Println("got end of response marker", streamId)
			close(m.streams[streamId])
			continue
		}
		m.streams[streamId] <- bb[4:]
	}
	log.Panic("read loop exited")
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     checkSameOrigin,
}

func checkSameOrigin(r *http.Request) bool {
	origin := r.Header["Origin"]
	log.Println("checking origin", origin)
	if len(origin) == 0 {
		log.Println("no origin, return true")
		return true
	}
	u, err := url.Parse(origin[0])
	if err != nil {
		log.Println("error parsing origin", err)
		return false
	}
	log.Printf("parsed origin [%+v] \n", u)
	log.Println("testig origins", u.Host, r.Host)
	return equalASCIIFold(u.Host, r.Host)
}

// equalASCIIFold returns true if s is equal to t with ASCII case folding as
// defined in RFC 4790.
func equalASCIIFold(s, t string) bool {
	for s != "" && t != "" {
		sr, size := utf8.DecodeRuneInString(s)
		s = s[size:]
		tr, size := utf8.DecodeRuneInString(t)
		t = t[size:]
		if sr == tr {
			continue
		}
		if 'A' <= sr && sr <= 'Z' {
			sr = sr + 'a' - 'A'
		}
		if 'A' <= tr && tr <= 'Z' {
			tr = tr + 'a' - 'A'
		}
		if sr != tr {
			return false
		}
	}
	return s == t
}

// this doesn't work
// use gorilla websocket instead
func (m *multiplexer) UpgradeToWebsocket(w http.ResponseWriter, r *http.Request) {
	// get the session id from the request
	// create a new session
	// write the session id to the response
	// start a goroutine to read from the websocket and write to the session
	// start a goroutine to read from the session and write to the websocket
	// return
	sessionId := m.newSession()
	log.Println("new session being upgraded to ws", sessionId)
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("error upgrading to websocket", err)
		return
	}
	conn.WriteMessage(websocket.TextMessage, []byte("hello"))
	log.Println("wrote session id to websocket", sessionId)

	pr, err := http.NewRequest(r.Method, r.URL.String(), r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprintln(w, "error creating proxy request", err)
		return
	}
	pr.Header = r.Header
	reqBody := encodeRequest(pr)
	reqBody = append(reqBody, '\n')
	m.Write(sessionId, reqBody)
	_, err = m.ReadLine(sessionId)
	if err != nil {
		log.Println("error pushing websocket request to edge", err)
	}

	go func() {
		log.Println("starting websocket read loop", sessionId)
		for {
			_, bb, err := conn.ReadMessage()

			if err == io.EOF {
				continue
			} else if err != nil {
				log.Println("error reading from websocket", err)
				return
			}
			log.Println("read from websocket", sessionId, string(bb))
			bb = append(bb, '\n')
			m.Write(sessionId, bb)
		}
	}()

	for {
		bb, err := m.ReadLine(sessionId)
		if err != nil {
			log.Println("error reading from session", err)

			return
		}
		log.Println("read from session", sessionId, string(bb))
		conn.WriteMessage(websocket.TextMessage, bb)
	}

}

func encodeId(id int) []byte {
	bb := make([]byte, 4)
	bb[0] = byte(id >> 24)
	bb[1] = byte(id >> 16)
	bb[2] = byte(id >> 8)
	bb[3] = byte(id)
	return bb
}

func decodeId(bb []byte) int {
	return int(bb[0])<<24 | int(bb[1])<<16 | int(bb[2])<<8 | int(bb[3])
}

func (m *multiplexer) Write(id int, data []byte) error {

	data = append(encodeId(id), data...)
	_, err := m.conn.Write(data)
	return err
}

func (m *multiplexer) ReadAll(id int) ([]byte, error) {
	response := []byte{}
	stream, exists := m.streams[id]
	if !exists {
		return nil, fmt.Errorf("stream not found %d", id)
	}
	for line := range stream {
		log.Println("stream", id, "got line", string(line))
		response = append(response, line...)
	}
	log.Println("stream", id, "closed")
	return response, nil
}

func (m *multiplexer) ReadLine(id int) ([]byte, error) {
	if stream, exists := m.streams[id]; exists {
		line := <-stream
		return line, nil
	}
	return nil, fmt.Errorf("stream not found %d", id)
}

func (m *multiplexer) newSession() int {
	id := int(atomic.AddInt32(&m.nextSessionId, 1))
	m.lock.Lock()
	m.streams[id] = make(chan []byte)
	m.lock.Unlock()
	return id
}

func (m *multiplexer) Do(req *http.Request) (http.Response, error) {
	id := m.newSession()
	reqBody := encodeRequest(req)
	reqBody = append(reqBody, '\n')
	m.Write(id, reqBody)
	respBody, err := m.ReadAll(id)
	if err != nil {
		return http.Response{}, err
	}
	resp := decodeResponse(respBody)
	return resp, nil
}

func encodeResponse(resp *http.Response) []byte {
	log.Println("encodeResponse", resp.Status)
	data := map[string]interface{}{}
	data["header"] = resp.Header
	data["status"] = resp.Status
	data["statusCode"] = resp.StatusCode
	if resp.Body != nil {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		data["body"] = base64.StdEncoding.EncodeToString(body)
	}
	bb, _ := json.Marshal(data)
	return bb
}

func decodeResponse(bb []byte) http.Response {
	data := map[string]interface{}{}
	json.Unmarshal(bb, &data)
	resp := http.Response{}
	resp.Header = http.Header{}
	for k, v := range data["header"].(map[string]interface{}) {
		vv := v.([]interface{})
		if len(vv) > 0 {
			for _, val := range vv {
				resp.Header.Add(k, val.(string))
			}
		}
	}
	resp.Status = data["status"].(string)
	resp.StatusCode = int(data["statusCode"].(float64))
	if _, exists := data["body"]; exists {

		body, _ := base64.RawStdEncoding.DecodeString(data["body"].(string))
		resp.Body = io.NopCloser(bytes.NewReader(body))
	}
	return resp
}

func encodeRequest(req *http.Request) []byte {
	resp := map[string]interface{}{}
	resp["method"] = req.Method
	resp["url"] = req.URL.String()
	resp["header"] = req.Header
	if req.Body != nil {
		body, _ := io.ReadAll(req.Body)
		resp["body"] = base64.StdEncoding.EncodeToString(body)
	}
	bb, _ := json.Marshal(resp)
	return bb
}

func decodeRequest(bb []byte) *http.Request {
	data := map[string]interface{}{}
	err := json.Unmarshal(bb, &data)
	if err != nil {
		log.Println("error decoding request", err, "\n", string(bb))
		return nil
	}
	req := &http.Request{}
	log.Println("decoding request", data)
	req.Method = data["method"].(string)
	req.URL, _ = url.Parse(data["url"].(string))
	req.Header = http.Header{}
	req.URL.Scheme = "http"

	header := data["header"].(map[string]interface{})
	for k, v := range header {
		switch v.(type) {
		case []interface{}:
			for _, val := range v.([]interface{}) {
				req.Header.Add(k, val.(string))
			}
		case string:
			req.Header.Add(k, v.(string))

		}
	}
	if _, exists := data["body"]; exists {
		body, _ := base64.RawStdEncoding.DecodeString(data["body"].(string))
		req.Body = io.NopCloser(bytes.NewReader(body))
	}
	return req
}
