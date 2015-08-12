# metaserv
Cluster Metadata Service using etcd

## What?
Run it on as many nodes as you like in your etcd cluster, and you will have a continuously up-to-date accounting of all nodes' metadata. Get it all by requesting `/meta`.

Each node discovers its own metadata and publishes that metadata to etcd every so often with a ttl of twice every so often. Every node watches all the nodes metadata keys and aggregates the results ready to be served via http.

## Getting
`go get github.com/soellman/metaserv`

## Contributing
Contributions are welcome!

## Surprise!
Right now, it doesn't actually discover any metadata, unless you like random numbers. The rest of it works though.