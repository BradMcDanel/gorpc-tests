package main

import (
    "fmt"
    "os"
    "net/rpc"
    "net"
    "time"
    "net/http"
    "strconv"
)

type DynArg struct {
    A []byte
}

type Message int


func (t *Message) Echo(args *DynArg, reply *DynArg) error {
    reply.A = args.A 
    return nil
}

func basicCall(c *rpc.Client, numBytes int) {
    args := DynArg{A: make([]byte, numBytes) };
    var reply DynArg
    err := c.Call("Message.Echo", args, &reply)
    checkError(err)
}

func checkError(err error) {
    if err != nil {
        fmt.Println("Fatal error ", err.Error())
        os.Exit(1)
    }
}

func startTCPServer(port string) (*rpc.Server) {
    tcpAddr, err := net.ResolveTCPAddr("tcp", ":" +  port)
    checkError(err)

    listener, err := net.ListenTCP("tcp", tcpAddr)
    checkError(err)

    newServer := rpc.NewServer()

    message := new(Message)
    newServer.Register(message)

    go newServer.Accept(listener)

    return newServer
}

func startHTTPServer(port string) (*rpc.Server) {
    newServer := rpc.NewServer()
    newServer.HandleHTTP(rpc.DefaultRPCPath, rpc.DefaultDebugPath)
    
    message := new(Message)
    newServer.Register(message)
    
    l, e := net.Listen("tcp", port)
    checkError(e)

    go http.Serve(l, nil)
	
    return newServer
}

func startTCPClient(serverAddress string, port string) (*rpc.Client) {
    client, err := rpc.Dial("tcp", serverAddress + ":" + port)
    checkError(err)
    
    return client
}

func startHTTPClient(serverAddress string, port string) (*rpc.Client) {
    client, err := rpc.DialHTTP("tcp", serverAddress + ":" + port)
    checkError(err)
    return client
}

func throughputTest(serverAddr string, numClients int, numServers int, numWindows int, messageSize int) {
    
    //arrays of clients and servers
    var clients []*rpc.Client = make([]*rpc.Client, numClients)
    var servers []*rpc.Server = make([]*rpc.Server, numServers)

    //starting port
    var startPort int = 4000

    //start servers
    for i := 0; i < numServers; i++ {
        servers[i] = startTCPServer(strconv.Itoa(startPort + i))
    }


    startTime := time.Now()

    //connect clients to servers
    for i := 0; i < numClients; i++ {
        //distribute clients evenly to servers
        clients[i] = startTCPClient(serverAddr, strconv.Itoa(startPort + (i % numServers)))

    }


    //send messages
    for i := 0; i < numWindows; i++ {
        for j := 0; j < numClients; j++ {
            basicCall(clients[j], messageSize)            
        }
    }       

    duration := time.Since(startTime)

    var throughputMb = float64(messageSize*8*numWindows*numClients)/duration.Seconds()/1000000

    fmt.Printf("Total time: %v s\n", duration.Seconds())
    fmt.Printf("Throughput (Mbits/s): %v\n", throughputMb)

}

func main() {
    if len(os.Args) != 2 {
        fmt.Println("Usage: ", os.Args[0], "[server]")
        os.Exit(1)
    }

    serverAddr := os.Args[1]

    //args - serverAddr, numClients, numServers, numWindows, msgSize in bytes
    throughputTest(serverAddr, 300, 1, 50, 10000)
}


