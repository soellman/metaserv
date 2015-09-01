package main

import (
	"bytes"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"strconv"
	"strings"
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
	Etcd{},
	Fleet{},
	Uname{},
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
	data := ds.Generate()
	if data == nil {
		log.Printf("Datasource %q failed\n", ds.Name())
		return
	}
	log.Printf("Datasource %q executed successfully\n", ds.Name())
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

// etcd. Reads `etcdctl --version`
type Etcd struct{}

func (e Etcd) Name() string { return "etcd" }

func (e Etcd) Start(ctx context.Context, out chan Datum) {
	go dsExec(out, e)
}

func (e Etcd) Generate() interface{} {
	out, err := exec.Command("etcdctl", "--version").Output()
	if err != nil {
		log.Printf("`etcdctl --version` returned error.")
		return nil
	}
	r := bytes.NewBuffer(out)
	m := make(map[string]string)
	ComposeReader(r, m, MatchAndRemove("etcdctl version "), MapKey("version"))
	return m
}

// fleet. Reads `fleetctl --version`
type Fleet struct{}

func (f Fleet) Name() string { return "fleet" }

func (f Fleet) Start(ctx context.Context, out chan Datum) {
	go dsExec(out, f)
}

func (f Fleet) Generate() interface{} {
	out, err := exec.Command("fleetctl", "--version").Output()
	if err != nil {
		log.Printf("`fleetdctl --version` returned error.")
		return nil
	}
	r := bytes.NewBuffer(out)
	m := make(map[string]string)
	ComposeReader(r, m, MatchAndRemove("fleetctl version "), MapKey("version"))
	return m
}

// uname. Reads data from `uname`
type Uname struct{}

func (u Uname) Name() string { return "uname" }

func (u Uname) Start(ctx context.Context, out chan Datum) {
	go dsExec(out, u)
}

func (u Uname) Generate() interface{} {
	cmds := []struct {
		key  string
		flag string
	}{
		{"hostname", "-n"},
		{"arch", "-m"},
		{"kernel_name", "-s"},
		{"kernel_release", "-r"},
	}

	m := make(map[string]string)
	for _, cmd := range cmds {
		out, _ := exec.Command("uname", cmd.flag).Output()
		m[cmd.key] = strings.TrimSpace(string(out))
	}
	return m
}
