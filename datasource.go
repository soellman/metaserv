package main

import (
	"log"
	"math/rand"
	"strconv"
	"time"

	"golang.org/x/net/context"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func randomKey() interface{} {
	i := rand.Int() % 1000
	debugf("generated random key %d\n", i)
	return map[string]string{"key": strconv.Itoa(i)}
}

func datasource(ctx context.Context, name string, out chan Datum, interval time.Duration, f func() interface{}) {
	log.Printf("Datasource %q started with interval %v\n", name, interval)
	ticker := time.NewTicker(interval)

	for {
		select {
		case <-ctx.Done():
			debugf("datasource %q cancelled\n", name)
			return
		case <-ticker.C:
			debugf("datasource %q ticker ticked\n", name)
			debugf("datasource %q sending data\n", name)
			out <- Datum{key: name, value: f()}
		}
	}
}

func datasources(ctx context.Context, out chan Datum) {
	go datasource(ctx, "random", out, 3*time.Second, randomKey)
}
