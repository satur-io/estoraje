# Estoraje - Key Value distributed database

Estoraje is the simplest distributed system for key-value storage. It is temporary consistent -but quite close to hard consistency-, high available, lightweight, scalable and gives a good performance.

You just need a load balancer on the top and your estoraje's nodes. No external service mesh coordination (like Consul or Zookeeper) is needed. It uses a consistent hashing algorithm to distribute and replicate the data among the nodes and a embed etcd server for coordination.

## Description

This project is developed for self training purposes. My main goal is building a simple although working system from scratch trying to avoid as much as possible using third parties packages. It is inspired by some other existing and more complex products and follows some well-known approaches for building data intensive applications. For more info in this topic I encourage recommend Martin Kleppmann's _Designing Data-Intensive Applications_ book. Most of the code is only in two files: One for the consistent hashing algorithm and other one with less than 1000 lines that contains the whole logic.

### Architecture diagram


### ¿How does it work?
Animated diagram

### The key-value storage

### The distribution and replication 

### The coordination among nodes

### ¿Why all logic in one file?


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