/* DOESN'T WORK YET
 * Sets up a user-defined number of servers and connects clients,
 * each client connected to one server, each server handling multiple clients
 * 
 * Basic usage:
 * go install gorpc-tests/variableMessage
 * variableMessage  [-ns number of servers]
 					[-nc number of clients]
 					[-ml message length]
 					[-nm number of messages]
 */

package main

import (	
    "fmt"
    "net"
    "net/rpc"
    "flag"
//    "math/rand"
    "log"
    "io/ioutil"
    "time"
    "os"
//    "bytes"
)

const (
	DEFAULTSERVER = "localhost"
    PORTBASE = 9000
    DEBUG = true
)

var numMessages int
var numClients int
var numServers int
var messageLength int

type ByteArgs struct {
	A []byte
}

type Arith int

func (t *Arith) Echo(args *ByteArgs, reply *ByteArgs) error {
	log.Printf("Initially %v, with length %d, reply is %v has length %d\n",
		 args.A, len(args.A), reply.A, len(reply.A))
	numWritten := copy(reply.A, args.A)
	log.Printf("Echo copied %d elems over", numWritten)
	return nil
}

func asynchronousCall(c *rpc.Client) {
	var args ByteArgs
	slice := make([]byte, messageLength, messageLength)
	for i := range slice {
        slice[i] = byte(i)
    }
	args.A = slice
	
	log.Printf("Args for asynchronousCall %v\n", args.A)

	// Asynchronous call
	var calls []*rpc.Call

	var reply ByteArgs
	returnslice := make([]byte, messageLength, messageLength)
	reply.A = returnslice
	for i := 0 ; i < numMessages; i++ {
		call := c.Go("Arith.Echo", args, &reply, nil)
		calls = append(calls, call)
	}

	for _, call := range calls {
        <-call.Done
    }

    log.Printf("Finished calls, message: %v\n", reply);
	//replyCall := <-divCall.Done	// will be equal to divCall
	// check errors, print, etc.
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
    flag.Parse()
    numServers = *nS
    numClients = *nC
    numMessages = *nM
    messageLength = *mL

	for i := 0; i < numServers; i++ {
		startTCPServer(PORTBASE + i)				
	}
	fmt.Printf("Started %d servers", numServers)

	var clients []*rpc.Client

	for i := 0; i < numClients; i++ {
		client := startTCPClient(PORTBASE + (i % numServers))
		clients = append(clients, client)
	}
	log.Printf("Started %d clients", numClients)

	for i := 0; i < numClients; i++ {
		go asynchronousCall(clients[i])
	}
	time.Sleep(1 * 1e9)
	//<-make(chan int)
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
		log.Println("Fatal error ", err.Error())
		os.Exit(1)
	}
}
