/* Windowed throughput
 *
 * Starts up a user-defined number of servers and connects clients (each client to one server).
 * Each client then sends windowed messages of a user-defined length to the server
 *
 * Basic usage:
 * go install gorpc-tests/windowedThroughput
 * windowedThroughput  [-ns number of servers]
 					[-nc number of clients]
 					[-ml message length]
 					[-nm number of messages one client should send]
 					[-ws window size]
 */

package main

import (	
    "fmt"
    "net"
    "net/rpc"
    "flag"
    "log"
    "io/ioutil"
    "time"
    "os"
	"sync"
)

const (
	DEFAULTSERVER = "localhost"
    PORTBASE = 9000
    DEBUG = false
)

var numMessages int
var numClients int
var numServers int
var messageLength int
var windowSize int

//argument that allows for variable length message
type ByteArgs struct {
	A []byte
}

//response type
type LenArgs struct {
	A int
}

type Arith int

//copies input byte-slice to reply 
func (t *Arith) Echo(args *ByteArgs, reply *ByteArgs) error {
	replyData := make([]byte, len(args.A))
	numWritten := copy(replyData, args.A)
	reply.A = replyData
	log.Printf("Echo copied %d elems over", numWritten)
	return nil
}

func (t *Arith) Echo2(args *ByteArgs, reply *ByteArgs) error {
	reply.A = args.A
	//fmt.Printf("Reply copied over %v\n", reply.A)
	return nil

}

//sets reply to the length of byte array sent in as argument
func (t *Arith) FindLen(args *ByteArgs, reply *LenArgs) error {
	reply.A = len(args.A)
	//fmt.Printf("Length of passed in message %d", reply.A)
	return nil
}

func worker(c *rpc.Client, args *ByteArgs, reply []*ByteArgs, linkChan chan int, clientGroup *sync.WaitGroup) {
	
	// Signal to client that the worker is complete when we leave the function
	defer clientGroup.Done()
	
	for v := range linkChan {
		c.Call("Arith.Echo", args, reply[v])
		//fmt.Println("Finished", v)
	}
}

//sends specified number of messages to server, with a designated window size
func clientWindowedCall(c *rpc.Client, w *sync.WaitGroup) {
	
	// Signal this client is complete when we leave the function
	defer w.Done()

	//create byte array that will serve as argument, and initialize with values
	var args ByteArgs
	slice := make([]byte, messageLength)
	for i := range slice {
        slice[i] = byte(i)
    }
	args.A = slice

	// space for windowSize replies
	var reply []*ByteArgs
	for i := 0 ; i < windowSize ; i++ {
		reply = append(reply, new(ByteArgs))
	}

	clientGroup := new(sync.WaitGroup)
	
	// The channel distributes work to the workers
	lCh := make(chan int)

	for i := 0 ; i < windowSize ; i++ {
			clientGroup.Add(1)
		go worker(c, &args, reply, lCh, clientGroup)
	}

	// Send in the work requests (number of messages) to the workers
	for i := 0; i < numMessages; i++ {
		lCh <- (i % windowSize)
	}

	close(lCh)

	//waits until all workers complete
	clientGroup.Wait()
}

func main() {
	if !DEBUG {
        // disables debug logging
        log.SetOutput(ioutil.Discard)
    }

	nS := flag.Int("ns", 1, "number of servers")
    nC := flag.Int("nc", 1, "number of clients")
    mL := flag.Int("ml", 100, "message length")
    nM := flag.Int("nm", 10, "number of messages a client should send")
    wS := flag.Int("ws", 1, "window size (# of outstanding messages)")
    flag.Parse()

    numServers = *nS
    numClients = *nC
    numMessages = *nM
    messageLength = *mL
    windowSize = *wS

    fmt.Printf("Num servers: %d, Num clients: %d, Num Messages Per Client: %d, Message Length: %d, Window Size: %d\n",
    	numServers, numClients, numMessages, messageLength, windowSize)

    //start servers
	for i := 0; i < numServers; i++ {
		startTCPServer(PORTBASE + i)				
	}
	fmt.Printf("Started %d server(s)\n", numServers)

	//starts clients
	var clients []*rpc.Client
	for i := 0; i < numClients; i++ {
		client := startTCPClient(PORTBASE + (i % numServers))
		clients = append(clients, client)
	}
	fmt.Printf("Started %d client(s)\n", numClients)

	//creates new group to wait until all clients are finished
	w := new(sync.WaitGroup)
	startTime := time.Now()
	for i := 0; i < numClients; i++ {
		w.Add(1)
		//asynchronously calls individual client to start sending messages
		go clientWindowedCall(clients[i], w)
	}

	w.Wait()

	totalTime := time.Since(startTime)
	totalMB := float64(messageLength * numMessages * numClients) / 1e6
	fmt.Printf("Total time: %v\n", totalTime)
	fmt.Printf("Total megabytes sent: %v\n", totalMB)
	var throughputMB = totalMB/totalTime.Seconds()
    fmt.Printf("Throughput (megabytes/s): %v\n", throughputMB)
}

//starts a TCP client connected to the designated port (with default server)
func startTCPClient(port int) (*rpc.Client) {
	log.Printf("Starting Client connecting to %v\n", DEFAULTSERVER + fmt.Sprintf(":%d", port))
	client, err := rpc.Dial("tcp", DEFAULTSERVER + fmt.Sprintf(":%d", port))
	checkError(err)

	return client
}

//starts a TCP server that accepts at the given port
func startTCPServer(port int) (*rpc.Server) {
	log.Printf("Starting server on port %d\n", port)
	tcpAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf(":%d", port))
	checkError(err)

	listener, err := net.ListenTCP("tcp", tcpAddr)
	checkError(err)

	newServer := rpc.NewServer()

	arith := new(Arith)
	newServer.Register(arith)

	log.Printf("Server at port %d trying to accept new connections", port)	
	go newServer.Accept(listener)
	//fmt.Println("Accepted new connection?")	
	return newServer
}

func checkError(err error) {
	if err != nil {
		fmt.Println("Fatal error ", err.Error())
		os.Exit(1)
	}
}
