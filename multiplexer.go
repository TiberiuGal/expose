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
		if err != nil {
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
	log.Panicln("read loop exited")
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
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	data["body"] = base64.StdEncoding.EncodeToString(body)
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
	body, _ := base64.RawStdEncoding.DecodeString(data["body"].(string))
	resp.Body = io.NopCloser(bytes.NewReader(body))
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
	for k, v := range data["header"].(map[string]interface{}) {
		req.Header.Add(k, v.(string))
	}
	if _, exists := data["body"]; exists {
		body, _ := base64.RawStdEncoding.DecodeString(data["body"].(string))
		req.Body = io.NopCloser(bytes.NewReader(body))
	}
	return req
}
