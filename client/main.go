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

var (
	localhost  string
	serverHost string
)

//serverAddr := "localhost:1044"

const defaultServerHost = "localhost:1044"
const statsServer = ":8028"

func init() {
	flag.StringVar(&localhost, "host", "http://localhost:8022", "local host address")
	flag.StringVar(&localhost, "host", defaultServerHost, "server address")
	flag.Parse()

}

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
	var wg sync.WaitGroup

	ss := newStatServer(statsServer, ctx)
	p := newProxy(localhost, "unu", "orem", ss.collect)
	if err := p.connect(ctx, grokHost); err != nil {
		return
	}
	wg.Add(1)
	go p.loop(ctx, &wg)

	go func() {
		<-ctx.Done()
		p.conn.Close()

	}()
	waitOnSignal(cancelFunc)
	wg.Wait()

}

func waitOnSignal(cancelFunc func()) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		log.Println("Interrupting data fixing ...")
		cancelFunc()

	}()
}
