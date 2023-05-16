package main

import (
	"log"
	"net"
	"net/http"

	"github.com/magiconair/properties"
	"github.com/tiberiugal/expose"
)

type config struct {
	InboundAddress      string
	EdgeListenerAddress string
}

func main() {
	props := properties.MustLoadFile("cloudy.env", properties.UTF8)
	var cfg config
	if err := props.Decode(&cfg); err != nil {
		log.Fatal(err)
	}

	s := expose.NewCloudServer()

	ln, err := net.Listen("tcp", cfg.EdgeListenerAddress)
	if err != nil {
		log.Fatal(err)
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				log.Println("error accepting connection", err)
				continue
			}
			err = s.Accept(conn)
			if err != nil {
				log.Println("error accepting connection", err)
				continue
			}
			log.Println("new connection accepted", conn.RemoteAddr())
		}
	}()

	log.Printf("starting incoming http server [%s] \n", cfg.InboundAddress)
	err = http.ListenAndServe(cfg.InboundAddress, s)
	log.Println("server closed", err)
}
