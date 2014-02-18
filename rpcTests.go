package main

import (
	"fmt"
	"os"
	"net/rpc"
	"net"
	"time"
	"net/http"
)

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

func startTCPClient(serverAddress string, port string) (*rpc.Client) {
	//fmt.Printf(serverAddress + port)
	client, err := rpc.Dial("tcp", serverAddress + port)
	checkError(err)

	return client
}

func startHTTPClient(serverAddress string, port string) (*rpc.Client) {
	//fmt.Printf(serverAddress + port)
	client, err := rpc.DialHTTP("tcp", serverAddress + port)
	checkError(err)

	return client
}

func synchronousCall(c *rpc.Client) {
	// Synchronous call
	args := Args{7,8}
	var reply int
	err := c.Call("Arith.Multiply", args, &reply)
	checkError(err)
	fmt.Printf("Arith: %d*%d=%d", args.A, args.B, reply)
}

//sends 1 byte to server and expects the same byte in return
func basicCall(c *rpc.Client) {
	args := BasicArg{1};
	var reply BasicArg
	err := c.Call("Arith.Echo", args, &reply)
	//fmt.Printf("Arith: %d, %d", args.A, reply.A)
	checkError(err)
}

func asynchronousCall(c *rpc.Client) {
	// Asynchronous call
	numCalls := 1000
	args := make([]Args, numCalls)

	for i := 0 ; i < numCalls ; i++ {
		args[i].A = i
		args[i].B = i+1
	}

	reply := make([]int, numCalls)

	for i := 0 ; i < numCalls ; i++ {
		c.Go("Arith.Wait", args[i], &reply[i], nil)
	}

	time.Sleep(100*time.Millisecond)

	//replyCall := <-divCall.Done	// will be equal to divCall
	// check errors, print, etc.
	for i := 0 ; i < numCalls ; i++ {
		fmt.Printf("Arith: %d to %d\n", args[i].B, reply[i])
	}
}

func startTCPServer(port string) (*rpc.Server) {

	tcpAddr, err := net.ResolveTCPAddr("tcp", port)
	checkError(err)

	listener, err := net.ListenTCP("tcp", tcpAddr)
	checkError(err)

	newServer := rpc.NewServer()

	arith := new(Arith)
	newServer.Register(arith)

	//fmt.Println("trying to accept new connection")	
	go newServer.Accept(listener)
	//fmt.Println("Accepted new connection?")	
	return newServer
}

func startHTTPServer(port string) (*rpc.Server) {

	newServer := rpc.NewServer()
	newServer.HandleHTTP(rpc.DefaultRPCPath, rpc.DefaultDebugPath)

	arith := new(Arith)
	newServer.Register(arith)
	
	l, e := net.Listen("tcp", port)
	checkError(e)

	go http.Serve(l, nil)
	//fmt.Println("Accepted new connection?")	
	return newServer
}

func basicCallTest(numCalls int, serverAddr string, port string, withHTTP bool) {
	var client1 *rpc.Client
	if withHTTP {
		fmt.Printf("Using HTTP\n")
		startHTTPServer(port)
		client1 = startHTTPClient(serverAddr, port)
	} else {
		fmt.Printf("Using basic TCP\n")
		startTCPServer(port)
		client1 = startTCPClient(serverAddr, port)
	}

	startTime := time.Now()
	for i := 0; i < numCalls ; i++ {
		basicCall(client1)
	}
	duration := time.Since(startTime)
	fmt.Printf("Average duration of Basic Call: %v us\n", duration.Seconds()*1000000/float64(numCalls))
}

func connectAndCloseClientTest(serverAddress string, port string, numClients int) {
	startTCPServer(port)
	startTime := time.Now()
	for i := 0; i < numClients ; i++ {
		client1 := startTCPClient(serverAddress, port)
		client1.Close()
	}
	duration := time.Since(startTime)
	fmt.Printf("Average duration to Connect + Close Client: %v us\n", 
		duration.Seconds()*1000000/float64(numClients) )
}

//will crash once there are too many clients (as long as that number is > numConnections)
func maxConnectionsTest(serverAddress string, port string, numConnections int) {
	startTCPServer(port)
	startTime := time.Now()
	for i := 0; i < numConnections ; i++ {
		fmt.Printf("Starting client # %d\n", i)
		startTCPClient(serverAddress, port)
		fmt.Printf("Successfully connected client # %d\n", i)
	}
	duration := time.Since(startTime)
	fmt.Printf("Duration to Connect %v Clients: %v us\n", 
		numConnections, duration.Seconds() )
}

func asyncCallTest(serverAddress string, port string, numCalls int) {
	startTCPServer(port)
	client1 := startTCPClient(serverAddress, port)
	startTime := time.Now()
	for i := 0; i < numCalls ; i++ {
		asynchronousCall(client1)
	}
	duration := time.Since(startTime)
	fmt.Printf("Average duration of Asynchronous Call: %v us\n", 
		duration.Seconds()*1000000/float64(numCalls) )
}

func main() {
	if len(os.Args) != 3 {
		fmt.Println("Usage: ", os.Args[0], "[server] [:port]")
		os.Exit(1)
	}

	serverAddr := os.Args[1]
	port := os.Args[2]

	basicCallTest(100000, serverAddr, port, false)
	//connectAndCloseClientTest(serverAddr, port, 10000)
	//maxConnectionsTest(serverAddr, port, 10000)
	//asyncCallTest(serverAddr, port, 1)
}

func checkError(err error) {
	if err != nil {
		fmt.Println("Fatal error ", err.Error())
		os.Exit(1)
	}
}