package main

import (
	"log"
	"time"
)

func main() {

	c := make(chan struct{})
	go func() {
		<-time.After(1*time.Second)
		c<- struct{}{}
	}()

	select {
	case <-time.After(2 * time.Second):
		log.Println("timout")
		case <- c:
			log.Println("completed")

	}

	log.Println("done")
}
