package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func main() {
	addr := flag.String("Addr", ":80", "address to listen on")
	flag.Parse()

	r := chi.NewMux()
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "hello")
		return
	})
	r.Get("/ws", func(w http.ResponseWriter, r *http.Request) {
		log.Println("got request", r.UserAgent())
		//w.WriteHeader(http.StatusOK)
		// upgrade to websocket
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			fmt.Println("error upgrading", err)
			return
		}
		defer conn.Close()
		err = conn.WriteMessage(websocket.TextMessage, []byte("hello from server"))
		if err != nil {
			fmt.Println("error writing message", err)
			return
		}
		conn.SetReadLimit(512)
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		conn.SetPongHandler(func(string) error { conn.SetReadDeadline(time.Now().Add(60 * time.Second)); return nil })

		for {
			log.Println("waiting for message")
			mt, msg, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("error: %v", err)
				}
				break
			}
			log.Println("got message", mt, string(msg))
			conn.WriteMessage(websocket.TextMessage, msg)
		}

	})
	r.Get("/chat", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, htmlPage)
		return
	})

	http.ListenAndServe(*addr, r)
}

const htmlPage = `
<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<title>Chat</title>
</head>
<body>
<h1>Chat</h1>
<input type="text" id="message" />
<button id="send">Send</button>
<div id="messages"></div>
<script>
var conn = new WebSocket("ws://demo-proxy.localhost:8080/ws");	
conn.onmessage = function(evt) {
	var messages = document.getElementById("messages");
	var msg = document.createElement("div");
	msg.innerHTML = evt.data;
	messages.appendChild(msg);
};
document.getElementById("send").addEventListener("click", function() {
	var msg = document.getElementById("message").value;
	conn.send(msg);
});
</script>
</body>
</html>


`
