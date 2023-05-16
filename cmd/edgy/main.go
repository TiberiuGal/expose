package main

import (
	"log"
	"net"

	"github.com/magiconair/properties"
	"github.com/tiberiugal/expose"
)

func main() {
	props := properties.MustLoadFile("edgy.env", properties.UTF8)
	var cfg edgeConfig
	if err := props.Decode(&cfg); err != nil {
		log.Fatal(err)
	}

	outboundConn, err := net.Dial("tcp", cfg.InboundServerAddr)
	if err != nil {
		log.Fatal(err)
	}
	defer outboundConn.Close()
	log.Println("Starting gclient")
	srv := expose.NewEdgeServer(cfg.LocalServerAddr, cfg.DesiredNamespace, outboundConn)
	srv.Run()
}

type edgeConfig struct {
	InboundServerAddr string
	LocalServerAddr   string
	DesiredNamespace  string `properties:"DesiredNamespace,default="`
}
