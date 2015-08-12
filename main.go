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

func write(data map[string]string) {
	meta["updated"] = time.Now().UTC().Format(time.RFC3339Nano)
	j, _ := json.Marshal(map[string]interface{}{"meta": meta, "values": data})
	writeEtcd(string(j))
}

func writer(ctx context.Context, in chan map[string]string) {
	for {
		select {
		case <-ctx.Done():
			debugf("writer cancelled\n")
			return
		case data := <-in:
			debugf("writer received %v\n", data)
			write(data)
		}
	}
}

func scheduler(ctx context.Context, in, out chan map[string]string) {
	data := make(map[string]string)
	ticker := time.NewTicker(interval)

	for {
		select {
		case <-ctx.Done():
			debugf("scheduler cancelled\n")
			ticker.Stop()
			return
		case <-ticker.C:
			debugf("scheduler ticker ticked\n")
			out <- data
		case data = <-in:
			debugf("scheduler received data: %v\n", data)
		}
	}
}

func merger(ctx context.Context, in, out chan map[string]string) {
	data := make(map[string]string)

	for {
		select {
		case <-ctx.Done():
			debugf("merger cancelled\n")
			return
		case newdata := <-in:
			debugf("merger received %v\n", newdata)
			for k := range newdata {
				data[k] = newdata[k]
			}
			out <- data
		}
	}
}

func workers() context.CancelFunc {
	var (
		writerChan    = make(chan map[string]string)
		schedulerChan = make(chan map[string]string)
		mergeChan     = make(chan map[string]string)

		keeperReqChan    = make(chan chan []byte)
		keeperSubmitChan = make(chan []byte)
		httpReqChan      = make(chan []byte)
	)

	ctx, cancel := context.WithCancel(context.Background())

	// Data collection pipeline
	go writer(ctx, writerChan)
	go scheduler(ctx, schedulerChan, writerChan)
	go merger(ctx, mergeChan, schedulerChan)
	go datasources(ctx, mergeChan)

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
	log.Printf("Starting.")

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
