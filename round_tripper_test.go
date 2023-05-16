package expose_test

import (
	// "bufio"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tiberiugal/expose"
)

func TestRoundTripper(t *testing.T) {
	buff := []byte(`HTTP/1.1 200 OK
Content-type: text/html; charset=UTF-8

Another one bytes the dust
` + expose.EndOfResponseMarker + "\r\n")

	conn := newCon(buff)
	p := expose.NewProxyConn(conn)

	t.Run("test simple setup", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/lorem", nil)
		assert.Nil(t, err, "unexpected error generating request")
		resp, err := p.RoundTrip(req)
		assert.Nil(t, err, "unexpected error after roundtrip")
		assert.Equal(t, 200, resp.StatusCode, "should match the mehod")
		assert.Equal(t, 1, len(resp.Header), "header size missmatched")

		body, err := io.ReadAll(resp.Body)
		assert.Nil(t, err, "unexpected error reading the body")
		assert.Equal(t, "Another one bytes the dust", string(body), "body missmatched")
	})
}

type conn struct {
	buff []byte
}

func (c *conn) Read(p []byte) (int, error) {
	copy(p, c.buff)
	return len(c.buff), nil
}

func (c *conn) Write(p []byte) (int, error) {
	return len(p), nil
}

func newCon(p []byte) *conn {
	return &conn{buff: p}
}
