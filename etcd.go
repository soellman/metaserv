package main

import (
	"encoding/json"
	"log"
	"time"

	"github.com/coreos/etcd/client"
	"golang.org/x/net/context"
)

func etcdConnect() (client.Client, error) {
	cfg := client.Config{
		Endpoints: []string{etcdPeer},
	}

	return client.New(cfg)
}

func initEtcd() {
	key := "/" + etcdKey

	c, err := etcdConnect()
	if err != nil {
		log.Fatalf("Can't connect to etcd: %v", err)
	}

	keysAPI := client.NewKeysAPI(c)

	gopts := client.GetOptions{Recursive: false, Sort: false, Quorum: true}
	r, err := keysAPI.Get(context.Background(), key, &gopts)
	if err != nil {
		switch err := err.(type) {
		case client.Error:
			if err.Code == 100 {
				// Not found - create it
				sopts := client.SetOptions{Dir: true}
				_, err := keysAPI.Set(context.Background(), key, "", &sopts)
				if err != nil {
					log.Fatalf("Error creating etcdKey dir: %v", err)
				}
				return
			}
		default:
			log.Fatalf("etcd error: %v", err)
		}
	}
	if !r.Node.Dir {
		log.Fatalf("Error: etcdKey %q is not a directory", key)
	}

}

// TODO: retry somewhere?
func writeEtcd(data string) {
	key := "/" + etcdKey + "/" + hostname

	c, err := etcdConnect()
	if err != nil {
		log.Printf("Can't connect to etcd: %v", err)
		return
	}

	keysAPI := client.NewKeysAPI(c)
	sopts := client.SetOptions{TTL: 2 * interval}
	_, err = keysAPI.Set(context.Background(), key, data, &sopts)
	if err != nil {
		log.Printf("Error setting etcd key %q: %v", key, err)
	}
}

func handleResponse(hosts map[string]interface{}, r *client.Response) map[string]interface{} {
	if r.Action == "delete" || r.Action == "expire" || r.Action == "compareAndDelete" {
		debugf("removing host\n")
		delete(hosts, r.Node.Key)
	} else if r.Action == "set" || r.Action == "update" || r.Action == "create" || r.Action == "compareAndSwap" {
		debugf("adding host\n")
		var data interface{}
		if err := json.Unmarshal([]byte(r.Node.Value), &data); err == nil {
			hosts[r.Node.Key] = data
		}
	}
	return hosts
}

func list(m map[string]interface{}) []interface{} {
	l := []interface{}{}
	for _, v := range m {
		l = append(l, v)
	}
	return l
}

func watcher(ctx context.Context, out chan []byte) {
	key := "/" + etcdKey
	meta := make(map[string]string)
	hosts := make(map[string]interface{})
	for {
		c, err := etcdConnect()
		if err != nil {
			log.Printf("Can't connect to etcd: %v", err)
			time.Sleep(time.Second)
		}
		w := client.NewKeysAPI(c).Watcher(key, &client.WatcherOptions{Recursive: true})

		for {
			r, err := w.Next(ctx)
			if err != nil {
				// This doesn't work, but should:
				// https://github.com/coreos/etcd/pull/3229
				if err == context.Canceled {
					debugf("watcher cancelled\n")
					return
				}

				// Until I figure that out, this will do the trick
				if clusterErr, ok := err.(*client.ClusterError); ok {
					if len(clusterErr.Errors) > 0 && clusterErr.Errors[0] == context.Canceled {
						debugf("watcher cancelled\n")
						return
					}
				}

				debugf("watcher got other etcd error: %v\n", err)
				break
			}
			hosts := handleResponse(hosts, r)
			meta["updated"] = time.Now().UTC().Format(time.RFC3339Nano)
			j, _ := json.Marshal(map[string]interface{}{"meta": meta, "hosts": list(hosts)})
			out <- j
		}
	}
}
