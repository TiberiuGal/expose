package main

import (
	"log"
	"net"
	"net/http"

	"github.com/magiconair/properties"
	"github.com/tiberiugal/expose"
)

type config struct {
	InboundAddress      string `properties:"inbound,default=:8080"`
	EdgeListenerAddress string `properties:"edge_listener,default=:1044"`
}

func main() {
	props, _ := properties.LoadAll([]string{"cloudy.env"}, properties.UTF8, true)
	var cfg config
	if err := props.Decode(&cfg); err != nil {
		log.Fatal(err)
	}

	s := expose.NewCloudServer()

	edgeListener, err := net.Listen("tcp", cfg.EdgeListenerAddress)
	if err != nil {
		log.Fatal(err)
	}
	go func() {
		for {
			conn, err := edgeListener.Accept()
			if err != nil {
				log.Println("error accepting connection", err)
				continue
			}
			err = s.AcceptEdgeConnection(conn)
			if err != nil {
				log.Println("error oppening tunel ", err)
				continue
			}
		}
	}()

	log.Printf("starting http server [%s] \n", cfg.InboundAddress)
	err = http.ListenAndServe(cfg.InboundAddress, s)
	log.Println("server closed", err)
}
