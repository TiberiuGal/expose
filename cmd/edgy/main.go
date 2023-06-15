package main

import (
	"log"
	"net"

	"github.com/magiconair/properties"
	"github.com/tiberiugal/expose"
)

func main() {
	props, _ := properties.LoadAll([]string{"edgy.env", "/etc/expose/edgy.env"}, properties.UTF8, true)
	var cfg edgeConfig
	if err := props.Decode(&cfg); err != nil {
		log.Fatal(err)
	}
	log.Println("cfg", cfg)
	cloudConnection, err := net.Dial("tcp", cfg.CloudServerAddr)
	if err != nil {
		log.Fatal(err)
	}
	defer cloudConnection.Close()

	srv := expose.NewEdgeServer(cfg.LocalServerAddr, cfg.Hostname, cloudConnection)
	srv.Run()
}

type edgeConfig struct {
	CloudServerAddr string `properties:"cloud,default=localhost:1044"`
	LocalServerAddr string `properties:"local,default=localhost:80"`
	Hostname        string `properties:"hostname,default="`
}
