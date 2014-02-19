package main

import "bytes"
import "crypto/md5"
import "errors"
import "flag"
import "fmt"
import "log"
import "net"
import "net/rpc"
import "os"
import "runtime"
import "sync"
import "code.google.com/p/snappy-go/snappy"

////
type DFS int

type DataChunk struct {
	Chunk, Hash []byte
}

func (d *DFS) GetBlock(blockSize int, reply *DataChunk) error {
	file, err := os.Open("moby.txt")
	defer file.Close()
	handleError(err)
	//
	data := make([]byte, blockSize)
	bytesRead, err := file.Read(data)
	// Trim the byte buffer if we read less
	if bytesRead < blockSize {
		data = data[:bytesRead]
	}
	handleError(err)
	//
	h := md5.New()
	h.Write(data)
	//
	reply.Chunk = data
	reply.Hash = h.Sum(reply.Hash)
	return nil
}

func (d *DFS) GetSnappyBlock(blockSize int, reply *DataChunk) error {
	err := d.GetBlock(blockSize, reply)
	handleError(err)
	//
	reply.Chunk, err = snappy.Encode(reply.Chunk, reply.Chunk)
	handleError(err)
	//
	return nil
}

////

func handleError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

////

func startServer(port int) *rpc.Server {
	tcpAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf(":%d", port))
	handleError(err)
	//
	listener, err := net.ListenTCP("tcp", tcpAddr)
	handleError(err)
	//
	rpcServer := rpc.NewServer()
	rpcServer.Register(new(DFS))
	//
	fmt.Println("Starting blocking server...")
	rpcServer.Accept(listener)
	return rpcServer
}

func startClient(host string, port int) *rpc.Client {
	client, err := rpc.Dial("tcp", host+fmt.Sprintf(":%d", port))
	handleError(err)

	return client
}

////

func performGetBlock(host string, port int, isSnappy bool) {
	remote := startClient(host, port)
	defer remote.Close()
	//
	var reply DataChunk
	var err error
	blockSize := 512 * 1024 // 512 KB
	// Retrieve the block
	if isSnappy {
		err = remote.Call("DFS.GetSnappyBlock", blockSize, &reply)
		handleError(err)
		reply.Chunk, err = snappy.Decode(reply.Chunk, reply.Chunk)
		handleError(err)
	} else {
		err = remote.Call("DFS.GetBlock", blockSize, &reply)
		handleError(err)
	}
	// Calculate the MD5 hash and ensure it's equal
	h := md5.New()
	h.Write(reply.Chunk)
	if !bytes.Equal(reply.Hash, h.Sum(nil)) {
		handleError(errors.New("Hash did not match"))
	}
}

func worker(host string, port int, isSnappy bool, linkChan chan int, w *sync.WaitGroup) {
	// Signal this is complete when we leave the function
	defer w.Done()
	//
	for _ = range linkChan {
		performGetBlock(host, port, isSnappy)
	}
}

////

func main() {
	// Allow Go to use more than one core
	runtime.GOMAXPROCS(runtime.NumCPU())
	//
	host := flag.String("host", "localhost", "Host IP or name")
	port := flag.Int("port", 1337, "Port number [default: 1337]")
	isServer := flag.Bool("server", false, "Run as server")
	isSnappy := flag.Bool("snappy", false, "Blocks encoded using Snappy codec")
	totalCalls := flag.Int("calls", 3000, "Number of calls to make to the server")
	flag.Parse()
	//
	if *isServer {
		// Start server blocks
		startServer(*port)
	} else {
		lCh := make(chan int)
		w := new(sync.WaitGroup)
		// Set up the worker pool
		for i := 0; i < 10; i++ {
			w.Add(1)
			go worker(*host, *port, *isSnappy, lCh, w)
		}
		// Send in the work requests to the workers
		for i := 0; i < *totalCalls; i++ {
			lCh <- i
		}
		close(lCh)
		w.Wait()
	}
}
