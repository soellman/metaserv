package main

import (
	"log"
	"math/rand"
	"strconv"
	"time"

	"golang.org/x/net/context"
)

type DataSource interface {
	Start(ctx context.Context, out chan Datum)
	Name() string
	Generate() interface{}
}

var sources = []DataSource{Random{}}

// Random. not very useful
type Random struct{}

func (r Random) Name() string { return "random" }

func (r Random) Start(ctx context.Context, out chan Datum) {
	rand.Seed(time.Now().UnixNano())
	go dsTicker(ctx, out, 3*time.Second, r)
}

func (r Random) Generate() interface{} {
	i := rand.Int() % 1000
	debugf("generated random key %d\n", i)
	return map[string]string{"key": strconv.Itoa(i)}
}

// dsTicker runs a ticker around a DataSource
func dsTicker(ctx context.Context, out chan Datum, interval time.Duration, ds DataSource) {
	log.Printf("Datasource %q started with interval %v\n", ds.Name(), interval)
	ticker := time.NewTicker(interval)

	for {
		select {
		case <-ctx.Done():
			debugf("datasource %q cancelled\n", ds.Name())
			return
		case <-ticker.C:
			debugf("datasource %q ticker ticked\n", ds.Name())
			debugf("datasource %q sending data\n", ds.Name())
			out <- Datum{key: ds.Name(), value: ds.Generate()}
		}
	}
}

func datasources(ctx context.Context, out chan Datum) {
	for _, ds := range sources {
		ds.Start(ctx, out)
	}
}
