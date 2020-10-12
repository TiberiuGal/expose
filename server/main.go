package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"runtime/pprof"
	"sync"
)
var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")
func main() {
	flag.Parse()
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	ctx, cancelFunc := context.WithCancel(context.TODO())
	wg := sync.WaitGroup{}
	wg.Add(1)
	proxy := newProxy()
	go proxy.ListenForClients(":1044")
	go serve(ctx, ":8123", &wg, proxy)
	log.Println("started listening")
	waitOnSignal(cancelFunc)

	wg.Wait()
}

func waitOnSignal(cancelFunc func()) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<- c
			log.Println("Interrupting data fixing ...")
			cancelFunc()

	}()
}
