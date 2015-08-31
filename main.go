package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/net/context"
	"gopkg.in/tylerb/graceful.v1"
)

type Datum struct {
	key   string
	value interface{}
}

func write(data interface{}) {
	meta["updated"] = time.Now().UTC().Format(time.RFC3339Nano)
	if j, err := json.Marshal(map[string]interface{}{"meta": meta, "data": data}); err == nil {
		writeEtcd(string(j))
	} else {
		log.Printf("ERROR: json marshalling failed: %v", err)
	}
}

// writer receives data and writes it
func writer(ctx context.Context, in chan interface{}) {
	for {
		select {
		case <-ctx.Done():
			debugf("writer cancelled\n")
			return
		case data := <-in:
			debugf("writer received %v\n", data)
			debugf("writer writing\n")
			write(data)
		}
	}
}

// scheduler receives data, sends it, and holds onto it
// scheduler sends held data to the writer on an interval
func scheduler(ctx context.Context, in, out chan interface{}) {
	var data interface{}
	ticker := time.NewTicker(interval)

	for {
		select {
		case <-ctx.Done():
			debugf("scheduler cancelled\n")
			ticker.Stop()
			return
		case <-ticker.C:
			debugf("scheduler ticker ticked\n")
			debugf("scheduler sending data\n")
			out <- data
		case data = <-in:
			debugf("scheduler received data: %v\n", data)
			debugf("scheduler sending data\n")
			out <- data
		}
	}
}

// aggregator aggregates data sources into a map and sends to the scheduler
func aggregator(ctx context.Context, in chan Datum, out chan interface{}) {
	data := make(map[string]interface{})

	for {
		select {
		case <-ctx.Done():
			debugf("aggregator cancelled\n")
			return
		case d := <-in:
			debugf("aggregator received data from %q\n", d.key)
			data[d.key] = d.value
			debugf("aggregator sending data\n")
			out <- data
		}
	}
}

func workers() context.CancelFunc {
	var (
		writerChan     = make(chan interface{})
		schedulerChan  = make(chan interface{})
		aggregatorChan = make(chan Datum)

		keeperReqChan    = make(chan chan []byte)
		keeperSubmitChan = make(chan []byte)
		httpReqChan      = make(chan []byte)
	)

	ctx, cancel := context.WithCancel(context.Background())

	// Data collection pipeline
	go writer(ctx, writerChan)
	go scheduler(ctx, schedulerChan, writerChan)
	go aggregator(ctx, aggregatorChan, schedulerChan)
	go datasources(ctx, aggregatorChan)

	// Data server pipeline
	go keeper(ctx, keeperSubmitChan, keeperReqChan)
	go watcher(ctx, keeperSubmitChan)

	http.HandleFunc("/meta", func(w http.ResponseWriter, r *http.Request) {
		keeperReqChan <- httpReqChan
		data := <-httpReqChan
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	})

	return cancel
}

// keeper receives the cluster metadata from the watcher
// keeper sends the data to the http handler on request
func keeper(ctx context.Context, in chan []byte, req chan chan []byte) {
	data := []byte{}
	for {
		select {
		case <-ctx.Done():
			debugf("keeper cancelled\n")
			return
		case data = <-in:
			debugf("keeper received data\n")
		case c := <-req:
			debugf("keeper got request\n")
			c <- data
		}
	}
}

var (
	debug    bool
	etcdKey  string
	etcdPeer string
	hostname string
	port     int
	interval time.Duration

	meta = make(map[string]string)
)

func parseFlags() {
	out, err := exec.Command("hostname", "-s").Output()
	host := "localhost"
	if err == nil {
		host = strings.TrimSpace(string(out))
	}
	flag.BoolVar(&debug, "debug", false, "debug logging")
	flag.StringVar(&etcdKey, "etcdKey", "meta", "etcd prefix")
	flag.StringVar(&etcdPeer, "etcdPeer", "http://localhost:2379", "etcd peer")
	flag.StringVar(&hostname, "hostname", host, "hostname override")
	flag.IntVar(&port, "port", 2235, "api port")
	flag.DurationVar(&interval, "interval", 60*time.Second, "etcd write interval. ttl is 2x interval")
	flag.Parse()
	meta["hostname"] = hostname
}

func debugf(format string, args ...interface{}) {
	if debug {
		log.Printf(format, args...)
	}
}

func main() {
	// Initialize everything
	parseFlags()
	initEtcd()
	log.Printf("Starting with hostname %q.", hostname)

	// Start up our worker routines
	cancel := workers()

	// Start up the webserver with a mechanism to wait for it later
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		graceful.Run(":"+strconv.Itoa(port), 1*time.Second, http.DefaultServeMux)
		wg.Done()
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigChan:
		log.Printf("Stopping.\n")
		cancel()
		wg.Wait()
		// Give the context time to cancel
		time.Sleep(100 * time.Millisecond)
	}
}
