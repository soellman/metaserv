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

func randomKey() map[string]string {
	i := rand.Int() % 1000
	log.Printf("generated random key %d\n", i)
	return map[string]string{"key": strconv.Itoa(i)}
}

func datasource(ctx context.Context, name string, out chan map[string]string, interval time.Duration, f func() map[string]string) {
	ticker := time.NewTicker(interval)

	for {
		select {
		case <-ctx.Done():
			debugf("datasource %q cancelled\n", name)
			return
		case <-ticker.C:
			debugf("datasource %q ticker ticked\n", name)
			out <- f()
		}
	}
}

func datasources(ctx context.Context, out chan map[string]string) {
	go datasource(ctx, "random", out, 3*time.Second, randomKey)
}
