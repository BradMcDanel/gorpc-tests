/* NUMBER OF MESSAGES CURRENTLY UNUSED (set to window size)
 * Sets up a user-defined number of servers and connects clients,
 * each client connected to one server, each server handling multiple clients,
 * with variable message size and message length
 *
 * Basic usage:
 * go install gorpc-tests/variableMessage
 * variableMessage  [-ns number of servers]
 					[-nc number of clients]
 					[-ml message length]
 					[-nm number of messages each client should send]
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

type ByteArgs struct {
	A []byte
}

type Arith int

//copies input byte-slice to reply 
func (t *Arith) Echo(args *ByteArgs, reply *ByteArgs) error {
	log.Printf("Initially %v, with length %d, reply is %v has length %d\n",
		 args.A, len(args.A), reply.A, len(reply.A))
	replyData := make([]byte, len(args.A))
	numWritten := copy(replyData, args.A)
	reply.A = replyData
	log.Printf("Echo copied %d elems over", numWritten)
	return nil
}

//sends -window number of calls to server, then waits for all responses
func asynchronousCall(c *rpc.Client, w *sync.WaitGroup) {
	
	// Signal this is complete when we leave the function
	defer w.Done()

	var args ByteArgs
	slice := make([]byte, messageLength)
	for i := range slice {
        slice[i] = byte(i)
    }
	args.A = slice
	
	log.Printf("Args for asynchronousCall %v\n", args.A)

	// Asynchronous call
	var calls []*rpc.Call

	// space for windowSize replies
	var reply []*ByteArgs

	for i := 0 ; i < windowSize; i++ {
		reply = append(reply, new(ByteArgs))
		//call := c.Go("Arith.Echo", args, &reply[i], nil)
		call := c.Go("Arith.Echo", args, reply[i], nil)
		calls = append(calls, call)
	}

	for _, call := range calls {
        <-call.Done
    }

    for i := 0 ; i < windowSize ; i++ {
    	log.Printf("Finished calls, message %d: %v\n", i, *reply[i]);
    }
}



func main() {
	if !DEBUG {
        // disables debug logging
        log.SetOutput(ioutil.Discard)
    }

	//rand.Seed(time.Now().Unix())

	nS := flag.Int("ns", 1, "number of servers")
    nC := flag.Int("nc", 1, "number of clients")
    mL := flag.Int("ml", 100, "message length")
    nM := flag.Int("nm", 1, "number of messages")
    wS := flag.Int("ws", 10, "window size (# of outstanding messages)")
    flag.Parse()

    numServers = *nS
    numClients = *nC
    numMessages = *nM
    messageLength = *mL
    windowSize = *wS

    fmt.Printf("Num servers: %d, Num clients: %d, Num Messages: %d, Message Length: %d, Window Size: %d\n",
    	numServers, numClients, numMessages, messageLength, windowSize)

	for i := 0; i < numServers; i++ {
		startTCPServer(PORTBASE + i)				
	}
	fmt.Printf("Started %d servers\n", numServers)

	var clients []*rpc.Client

	for i := 0; i < numClients; i++ {
		client := startTCPClient(PORTBASE + (i % numServers))
		clients = append(clients, client)
	}
	fmt.Printf("Started %d clients\n", numClients)

	w := new(sync.WaitGroup)
	startTime := time.Now()
	for i := 0; i < numClients; i++ {
		w.Add(1)
		go asynchronousCall(clients[i], w)
	}

	w.Wait()
	totalTime := time.Since(startTime)
	totalMbits := float64(messageLength * 8 * windowSize * numClients) / 1e6
	fmt.Printf("Total time: %v\n", totalTime)
	fmt.Printf("Total Mbits sent: %v\n", totalMbits)
	var throughputMb = totalMbits/totalTime.Seconds()
    fmt.Printf("Throughput (Mbits/s): %v\n", throughputMb)
}

func startTCPClient(port int) (*rpc.Client) {
	log.Printf("Starting Client connecting to %v", DEFAULTSERVER + fmt.Sprintf(":d", port))
	client, err := rpc.Dial("tcp", DEFAULTSERVER + fmt.Sprintf(":%d", port))
	checkError(err)

	return client
}

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
