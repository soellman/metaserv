package main

import (
	"bytes"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"strconv"
	"time"

	"golang.org/x/net/context"
)

type DataSource interface {
	Start(ctx context.Context, out chan Datum)
	Name() string
	Generate() interface{}
}

var sources = []DataSource{
	OSRelease{},
	Docker{},
}

func datasources(ctx context.Context, out chan Datum) {
	for _, ds := range sources {
		ds.Start(ctx, out)
	}
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

// dsExec runs a DataSource once
func dsExec(out chan Datum, ds DataSource) {
	log.Printf("Datasource %q executing", ds.Name())
	data := ds.Generate()
	if data == nil {
		log.Printf("Datasource %q failed\n", ds.Name())
		return
	}
	debugf("datasource %q sending data\n", ds.Name())
	out <- Datum{key: ds.Name(), value: ds.Generate()}
}

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

// os-release. Reads /etc/os-release
type OSRelease struct{}

func (o OSRelease) Name() string { return "os-release" }

func (o OSRelease) Start(ctx context.Context, out chan Datum) {
	go dsExec(out, o)
}

func (o OSRelease) Generate() interface{} {
	d := make(map[string]string)
	f, err := os.Open("/etc/os-release")
	if err != nil {
		log.Printf("/etc/os-release not read.")
		return nil
	}
	defer f.Close()
	ComposeReader(f, d, SplitOnString("="), TupleToMap())
	return d
}

// docker. Reads `docker info`
type Docker struct{}

func (d Docker) Name() string { return "docker" }

func (d Docker) Start(ctx context.Context, out chan Datum) {
	go dsExec(out, d)
}

func (d Docker) Generate() interface{} {
	out, err := exec.Command("docker", "version").Output()
	if err != nil {
		log.Printf("`Docker version` returned error.")
		return nil
	}
	r := bytes.NewBuffer(out)
	m := make(map[string]string)
	ComposeReader(r, m, SplitOnString(": "), TupleToMap())
	return m
}
