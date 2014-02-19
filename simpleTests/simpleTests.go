/* Provides three basic tests:  
 * 1) Serial null throughput: 1 client sending 1 byte, 1 server echoing that byte
 * 2) Maximum number of connections: Keep making clients until server can't handle any more
 * 3) Connect+Close Clients: 1 server, connect client, close client, repeat
 *
 * Basic usage:
 * go install gorpc-tests/basicTests
   basicTests [-port] [-test] [-http] [-nCalls] 
 */
   
package main

import (
	"fmt"
	"os"
	"net/rpc"
	"net"
	"time"
	"net/http"
	"io/ioutil"
	"flag"
	"log"
)

const (
	DEFAULTSERVER = "localhost"
    PORTBASE = 9000
    DEBUG = false
    NUMCONNECTIONS = 10000
    NUMCLIENTS = 10000
)

var port int
var withHTTP bool
var numCalls int

type Args struct {
	A, B int
}

type BasicArg struct {
	A byte
}

type Arith int

func (t *Arith) Multiply(args *Args, reply *int) error {
	*reply = args.A * args.B
	return nil
}

func (t *Arith) Echo(args *BasicArg, reply *BasicArg) error {
	reply.A = args.A 
	return nil
}

func (t *Arith) Wait(args *Args, reply *int) error {
	time.Sleep(time.Millisecond)
	*reply = (args.B + 1)
	return nil
}

func startTCPClient(port int) (*rpc.Client) {
	client, err := rpc.Dial("tcp", DEFAULTSERVER + fmt.Sprintf(":%d", port))
	checkError(err)

	return client
}

func startHTTPClient(port int) (*rpc.Client) {
	client, err := rpc.DialHTTP("tcp", DEFAULTSERVER + fmt.Sprintf(":%d", port))
	checkError(err)

	return client
}

func synchronousCall(c *rpc.Client) {
	// Synchronous call
	args := Args{7,8}
	var reply int
	err := c.Call("Arith.Multiply", args, &reply)
	checkError(err)
	log.Printf("Arith: %d*%d=%d", args.A, args.B, reply)
}

//sends 1 byte to server and expects the same byte in return
func basicCall(c *rpc.Client, w *sync.WaitGroup) {
	defer w.Done()
	args := BasicArg{1};
	var reply BasicArg
	err := c.Call("Arith.Echo", args, &reply)
	log.Printf("Arith: %d, %d", args.A, reply.A)
	checkError(err)
}

func startTCPServer(port int) (*rpc.Server) {

	tcpAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf(":%d", port))
	checkError(err)

	listener, err := net.ListenTCP("tcp", tcpAddr)
	checkError(err)

	newServer := rpc.NewServer()

	arith := new(Arith)
	newServer.Register(arith)
	
	go newServer.Accept(listener)
	return newServer
}

func startHTTPServer(port int) (*rpc.Server) {

	newServer := rpc.NewServer()
	newServer.HandleHTTP(rpc.DefaultRPCPath, rpc.DefaultDebugPath)

	arith := new(Arith)
	newServer.Register(arith)
	
	l, e := net.Listen("tcp", fmt.Sprintf(":%d", port))
	checkError(e)

	go http.Serve(l, nil)
	return newServer
}

func basicCallTest(port int) {
	var client1 *rpc.Client
	if withHTTP {
		log.Printf("Using HTTP\n")
		startHTTPServer(port)
		client1 = startHTTPClient(port)
	} else {
		log.Printf("Using basic TCP\n")
		startTCPServer(port)
		client1 = startTCPClient(port)
	}
	
	startTime := time.Now()
	for i := 0; i < numCalls ; i++ {
		w.Add()
		go basicCall(client1, w)
	}
	w.Wait()
	duration := time.Since(startTime)
	fmt.Printf("Average duration of Basic Call: %v us\n", duration.Seconds()*1000000/float64(numCalls))
}

func connectAndCloseClientTest(port int) {
	startTCPServer(port)
	startTime := time.Now()
	for i := 0; i < NUMCLIENTS ; i++ {
		client1 := startTCPClient(port)
		client1.Close()
	}
	duration := time.Since(startTime)
	fmt.Printf("Average duration to Connect + Close Client: %v us\n", 
		duration.Seconds()*1000000/float64(NUMCLIENTS) )
}

//will crash once there are too many clients (as long as that number is > numConnections)
func maxConnectionsTest(port int) {
	startTCPServer(port)
	startTime := time.Now()
	for i := 0; i < NUMCONNECTIONS ; i++ {
		fmt.Printf("Starting client # %d\n", i)
		startTCPClient(port)
		fmt.Printf("Successfully connected client # %d\n", i)
	}
	duration := time.Since(startTime)
	fmt.Printf("Duration to Connect %v Clients: %v us\n", 
		NUMCONNECTIONS, duration.Seconds() )
}

func main() {
	if !DEBUG {
        // disables debug logging
        log.SetOutput(ioutil.Discard)
    }

    p := flag.Int("port", PORTBASE, "port number")
    t := flag.Int("test", 1, "1 for basic, 2 for maxconnections, 3 for open+close connections")
    h := flag.Bool("http", false, "use HTTP")
    nCalls := flag.Int("nCalls", 100000, "number of calls to make")

    flag.Parse()
    port = *p
    withHTTP = *h
    test_type := *t
    numCalls = *nCalls
    switch test_type {
    	case 1 :
    		basicCallTest(port)
    	case 2 :
    		maxConnectionsTest(port)
    	case 3 :
    		connectAndCloseClientTest(port)
    }
}

func checkError(err error) {
	if err != nil {
		fmt.Println("Fatal error ", err.Error())
		os.Exit(1)
	}
}