package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
	"go.etcd.io/etcd/server/v3/embed"
	"go.etcd.io/etcd/server/v3/etcdserver/api/v3client"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/satur-io/estoraje/grpc"
	"github.com/satur-io/estoraje/lib/hashing"
)

const (
	Joining        int8 = 0
	Ready          int8 = 1
	Recovering     int8 = 2
	Reorganization int8 = 3
	Unconsistent   int8 = 4
	Down           int8 = -1

	grpcPort                    = 9000
	numberOfWritingDoneForValid = 1
	etcdDir                     = "etcd_conf"
)

var (
	nodeName       = flag.String("name", "node1", "The node name")
	host           = flag.String("host", "node1", "The host name")
	apiPort        = flag.String("port", "8000", "The API port")
	debug          = flag.Bool("debug", false, "Debug logs")
	filePath       = flag.String("dataPath", "data", "Data directory path")
	initialCluster = flag.String("initialCluster", "node1=http://node1:2380", "Etcd initial cluster hosts")
	addToCluster   = flag.Bool("add", false, "Add to existent cluster")

	isApiStarted  = false
	isGrpcStarted = false
	firstSyncDone = false

	readFile     = os.ReadFile
	writeFile    = os.WriteFile
	removeFile   = os.Remove
	lockKey      = concurrencyLockKey
	updateStatus = updateStatusEtcd

	ring    *hashing.Consistent
	newRing *hashing.Consistent

	etcdClient *clientv3.Client
	session    *concurrency.Session

	grpcConnections = make(map[string]grpc.ClientConn)
)

type Node struct {
	Host    string
	ApiHost string
	Status  int8
	Id      uint64
}

func joinCluster() {
	cfg := embed.NewConfig()
	cfg.Name = *nodeName
	cfg.LPUrls = parseUrls([]string{"http://0.0.0.0:2380"})
	cfg.LCUrls = parseUrls([]string{"http://0.0.0.0:2379"})
	cfg.APUrls = parseUrls([]string{fmt.Sprintf("http://%s:2380", *host)})
	cfg.ACUrls = parseUrls([]string{fmt.Sprintf("http://%s:2379", *host)})
	cfg.InitialClusterFromName("estoraje-cluster")

	if *addToCluster {
		addMemberToRemoteCluster()
		cfg.ClusterState = embed.ClusterStateFlagExisting
	}
	cfg.InitialCluster = *initialCluster
	cfg.Dir = fmt.Sprintf("%s/etcd3.%s", etcdDir, *nodeName)
	etcdServer, err := embed.StartEtcd(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer etcdServer.Close()
	etcdClient = v3client.New(etcdServer.Server)

	select {
	case <-etcdServer.Server.ReadyNotify():
		log.Printf("Etcd server is ready!")
		go addWatcher(etcdClient)

		session, _ = concurrency.NewSession(etcdClient)
		defer session.Close()

		status := Ready
		if *addToCluster {
			status = Joining
		}
		nodeInfo, _ := json.Marshal(Node{Host: fmt.Sprintf("%s:%d", *host, grpcPort), ApiHost: fmt.Sprintf("%s:%s", *host, *apiPort), Status: status, Id: uint64(etcdServer.Server.ID())})
		_, err := etcdClient.Put(context.TODO(), fmt.Sprintf("strj_node_%s", *nodeName), string(nodeInfo))
		if err != nil {
			log.Fatal(err)
		}

	case <-time.After(60 * time.Second):
		etcdServer.Server.Stop() // trigger a shutdown
		log.Printf("Etcd server took too long to start!")
	}
	log.Fatal(<-etcdServer.Err())
}

func addMemberToRemoteCluster() {
	nodes := strings.Split(*initialCluster, ",")
	var endpoints []string
	for _, node := range nodes {
		endpoints = append(endpoints, strings.Split(node, "=")[1])
	}

	// expect dial time-out on ipv4 blackhole
	_, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 2 * time.Second,
	})

	if err != nil {
		log.Fatal(err)
	}

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		log.Fatal(err)
	}

	memberlist, _ := cli.MemberList(context.Background())

	for _, member := range memberlist.Members {
		if member.PeerURLs[0] == fmt.Sprintf("http://%s:2380", *host) {
			cli.MemberRemove(context.Background(), member.ID)
		}
	}

	cli.MemberAdd(context.Background(), []string{fmt.Sprintf("http://%s:2380", *host)})
	defer cli.Close()
}

func updateStatusEtcd(node string, status int8) {
	var nodeInfo Node
	currentNode, err := etcdClient.Get(context.Background(), fmt.Sprintf("strj_node_%s", node))
	if err != nil {
		log.Fatal(err)
	}

	json.Unmarshal(currentNode.Kvs[0].Value, &nodeInfo)

	nodeInfo.Status = status
	nodeUpdated, _ := json.Marshal(nodeInfo)
	_, err = etcdClient.Put(context.TODO(), fmt.Sprintf("strj_node_%s", node), string(nodeUpdated))
	if err != nil {
		log.Fatal(err)
	}
}

func addWatcher(etcdClient *clientv3.Client) {
	rch := etcdClient.Watch(context.Background(), "strj_node_", clientv3.WithPrefix())

	for wresp := range rch {
		for range wresp.Events {
			log.Printf("Mesh config changed, loading ring")
			loadRing()
		}
	}
}

func startApiServer() {
	isApiStarted = true
	router := setupRouter()

	router.Run(":" + *apiPort)
}

func startGrpcServer() {
	isGrpcStarted = true
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", grpcPort))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterKVServer(s, &grpcServer{})
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func firstSync() {
	keys := make(chan string)

	go getRemoteKeys(keys)

	for key := range keys {
		hk, _ := newRing.GetNodes(key)

		for _, node := range hk.Nodes {
			if strings.HasPrefix(node, *host) {
				go updateFromRemote(key)
			}
		}
	}

	updateStatus(*nodeName, Ready)
}

func updateFromRemote(key string) {
	hk, _ := newRing.GetNodes(key)
	ch := dummyChannel[bool]()
	for _, node := range hk.Nodes {
		if !strings.HasPrefix(node, *host) {
			value, err := remoteRead(key, node)
			if err != nil {
				log.Fatal("Error syncronizing")
			}

			localWrite(key, value, ch)
		}
	}
	close(ch)
}

func setupRouter() *gin.Engine {
	if !*debug {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())

	if *debug {
		router.Use(gin.Logger())
	}

	router.GET("/:key", handleRead)
	router.POST("/:key", handleWrite)
	router.DELETE("/:key", handleWrite)
	router.GET("/_nodes_discovery", func(ctx *gin.Context) { ctx.JSON(http.StatusOK, readNodes()) })
	router.GET("/_nodes_discovery_plain", func(ctx *gin.Context) {
		for _, node := range readNodes() {
			ctx.String(http.StatusOK, fmt.Sprintf("%s\n", node.ApiHost))
		}
	})
	router.GET("/_health", func(ctx *gin.Context) { ctx.String(http.StatusOK, "") })
	router.GET("/_cluster_status", handleClusterStatus)
	router.GET("/_nodes/:key", handleNodes)
	router.DELETE("/_nodes/delete/:node", handleDeleteNode)

	return router
}

func handleClusterStatus(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	resp, err := etcdClient.Get(ctx, "strj_node_", clientv3.WithPrefix(), clientv3.WithSort(clientv3.SortByKey, clientv3.SortDescend))
	cancel()

	if err != nil {
		log.Fatal(err)
	}
	list, _ := etcdClient.Cluster.MemberList(context.Background())

	c.JSON(http.StatusOK, map[string]any{"keys": resp, "members": list})
}

func handleNodes(c *gin.Context) {
	key := c.Param("key")
	hk, _ := ring.GetNodes(key)
	nodes := hk.Nodes

	c.JSON(http.StatusOK, nodes)
}

func handleDeleteNode(c *gin.Context) {
	nodeId := c.Param("node")
	var nodeInfo Node
	node, err := etcdClient.KV.Get(context.Background(), fmt.Sprintf("strj_node_%s", nodeId))
	if err != nil {
		log.Fatal(err)
	}

	if len(node.Kvs) == 0 {
		c.String(http.StatusNotFound, "The node was not found")
		return
	}

	json.Unmarshal(node.Kvs[0].Value, &nodeInfo)

	etcdClient.KV.Delete(context.Background(), fmt.Sprintf("strj_node_%s", nodeId))
	etcdClient.Cluster.MemberRemove(context.Background(), nodeInfo.Id)
	c.String(http.StatusOK, "")
}

func handleRead(c *gin.Context) {
	key := c.Param("key")
	hk, _ := ring.GetNodes(key)
	nodes := hk.Nodes

	for i, node := range nodes {
		if strings.HasPrefix(node, *nodeName) && i > 0 {
			nodes[0], nodes[i] = nodes[i], nodes[0]
		}
	}

	for _, node := range nodes {
		var value []byte
		var error error

		if strings.HasPrefix(node, *host) {
			value, error = localRead(key)
		} else {
			value, error = remoteRead(key, node)
		}

		if error == nil {
			c.Data(http.StatusOK, "raw", value)
			return
		}
	}

	c.Data(http.StatusNotFound, "raw", []byte(""))
}

func concurrencyLockKey(key string) (unlock func() error, err error) {
	locker := concurrency.NewMutex(session, key)
	unlock = func() error {
		return locker.Unlock(context.Background())
	}
	err = locker.Lock(context.Background())
	return
}

func handleWrite(c *gin.Context) {
	key := c.Param("key")
	var value []byte

	if c.Request.Method == http.MethodPost {
		value, _ = c.GetRawData()
	}

	var localAction func()
	var remoteAction func(node string)

	unlock, err := lockKey(key)
	defer unlock()

	if err != nil {
		log.Printf("Error locking %s", key)
		c.Data(http.StatusInternalServerError, "raw", []byte("Error writing"))
		return
	}

	hk, _ := ring.GetNodes(key)
	nodes := hk.Nodes
	ch := make(chan bool)
	response := make(chan bool)

	switch method := c.Request.Method; method {
	case http.MethodPost:
		localAction = func() { localWrite(key, value, ch) }
		remoteAction = func(node string) { remoteWrite(key, value, ch, node) }
	case http.MethodDelete:
		localAction = func() { localDelete(key, ch) }
		remoteAction = func(node string) { remoteDelete(key, ch, node) }
	}

	for _, node := range nodes {
		if strings.HasPrefix(node, *host) {
			go localAction()
		} else {
			go remoteAction(node)
		}
	}

	manageWrites(ch, response, len(nodes))
	if <-response {
		c.Data(http.StatusOK, "raw", []byte(""))
		return
	} else {
		c.Data(http.StatusInternalServerError, "raw", []byte("Write failed on too much nodes, dissmissing..."))
		return
		//TODO: manage full write error
	}
}

func manageWrites(ch chan bool, response chan bool, expectedResponses int) {
	go func() {
		writes := 0
		responses := 0

		for {
			writed := <-ch
			responses++
			if writed {
				writes++
			} else {
				// TODO: manage node error
			}

			if writes >= numberOfWritingDoneForValid {
				response <- true
			}

			if responses >= expectedResponses {
				response <- false
			}
		}
	}()
}

func localRead(key string) (value []byte, err error) {
	value, err = readFile(fmt.Sprintf("%s/%s", *filePath, key))
	return
}

func localWrite(key string, value []byte, ch chan bool) {
	if err := writeFile(fmt.Sprintf("%s/%s", *filePath, key), value, 0600); err != nil {
		updateStatus(*nodeName, Unconsistent)
		ch <- false
		return
	}

	ch <- true
}

func localDelete(key string, ch chan bool) {
	if err := removeFile(fmt.Sprintf("%s/%s", *filePath, key)); err != nil {
		updateStatus(*nodeName, Unconsistent)
		ch <- false
		return
	}

	ch <- true
}

func remoteRead(key string, node string) (value []byte, err error) {
	client := getClient(node)
	val, err := client.Get(context.Background(), &pb.Key{Hash: key})
	if err == nil {
		value = val.Value
	}
	return
}

func remoteWrite(key string, value []byte, ch chan bool, node string) {
	client := getClient(node)
	result, err := client.Set(context.Background(), &pb.KeyValue{Key: key, Value: value})
	if err == nil && result.Ok {
		ch <- true
		return
	}

	ch <- false
}

func remoteDelete(key string, ch chan bool, node string) {
	client := getClient(node)
	result, err := client.Delete(context.Background(), &pb.Key{Hash: key})
	if err == nil && result.Ok {
		ch <- true
		return
	}

	ch <- false
}

func getClient(node string) pb.KVClient {
	connection, ok := grpcConnections[node]

	if ok {
		return pb.NewKVClient(&connection)
	}

	conn, err := grpc.Dial(node, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}

	grpcConnections[node] = *conn

	return pb.NewKVClient(conn)
}

func getRemoteKeys(keys chan string) {
	for _, node := range ring.Members() {
		if strings.HasPrefix(node, *host) {
			continue
		}
		client := getClient(node)
		stream, error := client.Keys(context.Background(), &pb.Empty{})
		if error != nil {
			log.Fatalf("read remote keys failed: %v", error)
		}

		for {
			key, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Fatalf("read remote keys failed: %v", err)
			}
			keys <- key.Hash
		}
	}
	close(keys)
}

func readKeys(ch chan string, quit chan bool) {
	files, err := ioutil.ReadDir(*filePath)
	if err != nil {
		quit <- true
		log.Fatal(err)
	}

	for _, file := range files {
		ch <- file.Name()
	}

	quit <- true
}

type grpcServer struct {
	pb.UnimplementedKVServer
}

func (s *grpcServer) Get(context context.Context, key *pb.Key) (*pb.Value, error) {
	value, err := localRead(key.GetHash())
	return &pb.Value{
		Value: value,
	}, err
}

func (s *grpcServer) Set(context context.Context, kv *pb.KeyValue) (*pb.Result, error) {
	if err := writeFile(fmt.Sprintf("%s/%s", *filePath, kv.Key), kv.Value, 0600); err != nil {
		return &pb.Result{Ok: false, Error: err.Error()}, err
	}

	return &pb.Result{Ok: true}, nil
}

func (s *grpcServer) Delete(context context.Context, kv *pb.Key) (*pb.Result, error) {
	if err := removeFile(fmt.Sprintf("%s/%s", *filePath, kv.Hash)); err != nil {
		return &pb.Result{Ok: false, Error: err.Error()}, err
	}

	return &pb.Result{Ok: true}, nil
}

func (s *grpcServer) Keys(empty *pb.Empty, stream pb.KV_KeysServer) error {
	ch := make(chan string)
	quit := make(chan bool)

	go readKeys(ch, quit)

	for {
		select {
		case key := <-ch:
			stream.Send(&pb.Key{Hash: key})
		case <-quit:
			return nil
		}
	}
}

func readNodes() (nodes []Node) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	resp, err := etcdClient.Get(ctx, "strj_node_", clientv3.WithPrefix(), clientv3.WithSort(clientv3.SortByKey, clientv3.SortDescend))
	cancel()

	if err != nil {
		log.Fatal(err)
	}

	for _, ev := range resp.Kvs {
		var node Node
		json.Unmarshal(ev.Value, &node)
		nodes = append(nodes, node)
	}

	return
}

func dummyChannel[T comparable]() (dummyChannel chan T) {
	dummyChannel = make(chan T)
	go func() {
		for range dummyChannel {
		}
	}()
	return
}

func startCleaner() {
	ch := make(chan string)
	quit := make(chan bool)
	dch := dummyChannel[bool]()
	deletes := 0

	go readKeys(ch, quit)

	for {
		select {
		case key := <-ch:
			hk, err := ring.GetNodes(key)

			if err != nil {
				log.Printf("Error cleaning the node")
				continue
			}

			if !contains(hk.Nodes, fmt.Sprintf("%s:%d", *host, grpcPort)) {
				deletes++
				localDelete(key, dch)
			}

		case <-quit:
			close(dch)
			log.Printf("%d key deleted", deletes)
			return
		}
	}
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if strings.EqualFold(s, needle) {
			return true
		}
	}
	return false
}

func parseUrls(values []string) []url.URL {
	urls := make([]url.URL, 0, len(values))
	for _, s := range values {
		u, err := url.Parse(s)
		if err != nil {
			log.Printf("Invalid url %s: %s", s, err.Error())
			continue
		}
		urls = append(urls, *u)
	}
	return urls
}

func makeDirs() {
	if _, err := os.Stat(*filePath); os.IsNotExist(err) {
		os.MkdirAll(*filePath, 0777)
	}

	if _, err := os.Stat(etcdDir); os.IsNotExist(err) {
		os.MkdirAll(etcdDir, 0777)
	}
}

func loadRing() {
	// TODO: manage reorganization
	r := hashing.New()
	nr := hashing.New()
	downscale := false

	for _, node := range readNodes() {
		switch node.Status {
		case Ready:
			log.Printf("Adding node %s to working ring", node.Host)
			r.Add(node.Host)
			nr.Add(node.Host)
		case Joining:
			log.Printf("Adding node %s to future ring", node.Host)
			nr.Add(node.Host)
		default:
			log.Printf("Node %s is not ready", node.Host)
		}
	}

	if ring != nil && len(ring.Members()) > len(r.Members()) {
		log.Printf("Downscaling")
		downscale = true
	}

	ring = r
	newRing = nr

	makeDirs()

	if !isGrpcStarted {
		log.Print("Starting Grpc server")
		go startGrpcServer()
	}

	if !isApiStarted {
		log.Print("Starting API server")
		go startApiServer()
	}

	if (*addToCluster && !firstSyncDone) || downscale {
		firstSyncDone = true
		firstSync()
	} else {
		log.Print("Starting cleaner")
		go startCleaner()
	}
}

func main() {
	flag.Parse()

	joinCluster()
}
