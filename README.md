# Estoraje ![Estoraje ring](docs/estoraje_ring.svg)
> Key Value distributed database

Estoraje is the simplest distributed system for key-value storage. It is temporary consistent -but quite close to hard consistency-, high available, lightweight, scalable and gives a good performance.

You just need a load balancer on the top and your estoraje's nodes. No external service mesh coordination (like Consul or Zookeeper) is needed. It uses a consistent hashing algorithm to distribute and replicate the data among the nodes and an embed etcd server for coordination.

## Quick start
There is a docker compose configuration and a `Makefile` for development.

* Running a cluster
```sh
make start
```

Take a look at other options running `make`.

Now you should be able to use estoraje. Using it is quite simple:

```sh
# Add a key/value
curl -X POST -d "{value}" http://localhost:8000/{key}

# Read a value
curl http://localhost:8000/{key}

# Delete key/value
curl -X DELETE http://localhost:8000/{key}
```

### Benchmarking
For a production-like cluster (keep in mind estoraje is not ready for production at all):

You can run a three nodes server this way

1. Install estoraje on each node [as any other go app](https://go.dev/doc/tutorial/compile-install).

2. Run estoraje
```sh
# Node 1
estorage -name=node_1 \
	-initialCluster=node_1=https://n1.satur.io:2380,node_2=https://n2.satur.io:2380,node_3=https://n3.satur.io:2380
	-host=n1.satur.io
	-port=8001
	-dataPath=data
	
# Node 2
estorage -name=node_2 \
	-initialCluster=node_1=https://n1.satur.io:2380,node_2=https://n2.satur.io:2380,node_3=https://n3.satur.io:2380
	-host=n2.satur.io
	-port=8001
	-dataPath=data
	
# Node 3
estorage -name=node_3 \
	-initialCluster=node_1=https://n1.satur.io:2380,node_2=https://n2.satur.io:2380,node_3=https://n3.satur.io:2380
	-host=n3.satur.io
	-port=8001
	-dataPath=data
```

You should also have a load balancer. You could install caddy and just run as reverse proxy load balancer

```sh
caddy reverse-proxy --from estoraje.satur.io --to n1.satur.io --to n2.satur.io --to n3.satur.io
```

Install estoraje in each node

## Description

This project is developed for self training purposes. My main goal is building a simple although working system from scratch trying to avoid as much as possible using third parties packages. It is inspired by some other existing and more complex products and follows some well-known approaches for building data intensive applications. For more info in this topic I encourage recommend Martin Kleppmann's _Designing Data-Intensive Applications_ book. Most of the code is only in two files: One for the consistent hashing algorithm and other one with less than 1000 lines that contains the whole logic.

### Architecture

We actually need two different features to make our system functional: On one side, we need to decide on an approach to distribute the data among the nodes, and on the other side, we must coordinate these nodes.
This could be done in some different ways: master-slave, consensus algorithm... To make a decision is essential to know what are we expecting from our system: ¿High availability? ¿Large storage? ¿Real-time? ¿Consistency? So, to simplify, we are assuming some outlines our use-case:

- All nodes should have the same responsibility and be, as far as possible, identical. Just one source code for each piece.
- We expect more reading than writing. Also, we expect that a handful of keys are requested more times than the other ones.
- It should be fast. Reading faster than Writing faster than Deleting.
- Temporary consistency is enough.
- We want to add and removes nodes with no downtime.
- Avoid using third-parties software.
- Most important, it should be as simple as possible.

![Architecture schema](docs/schema.png)

Taking in mind the acceptance criteria, this is the approach:

- Use consistent hashing to distribute the data among the nodes.
- Use a hard consistency system to coordinate the nodes. In our case, we have an embed etcd server as a sidecar on each node.

### ¿Why all logic in one file?
Almost all the code is in one file, `main.go`. Why?

This is a way to force myself to keep the system simple! If you can use only one file -and you don't want to go crazy- all non-essential code will be removed, and you won't develop unwanted features.

Finally, we have less than 800 lines and no plans to make many changes: the main goal -learning- was reached. The code is also more accessible this way, at least you can understand how are implemented most of the core concepts just taking a look for some minutes at the one-file.

## My learning goals:
- Distributed systems and data intensive applications
- gRPC
- Go basics 

### References:
- Designing Data-Intensive Applications: The Big Ideas Behind Reliable, Scalable, and Maintainable Systems
- [Etcd](https://etcd.io/ "A distributed, reliable key-value store for the most critical data of a distributed system ")
- [Minikeyvalue](https://github.com/geohot/minikeyvalue "~1000 line distributed key value store")
- [Consistent hashing](https://www.paperplanes.de/2011/12/9/the-magic-of-consistent-hashing.html "The Simple Magic of Consistent Hashing")
- [Profiling Go](https://github.com/DataDog/go-profiler-notes/blob/main/guide/README.md "The Busy Developer's Guide to Go Profiling, Tracing and Observability")