/** Iterated Paxos (based on Paxos A algorithm from class)
  * Runs through Paxos MAX_ITER times
  * Basic usage:
  * go install gorpc-tests/paxos
  * ./start.sh # starts acceptors
  * time paxos -prop # starts and times proposer
  *
  * To change the number of machines involved change the F constant below
  * and update start.sh to start up 2F acceptors 
  * (probably should killall paxos as well to kill old instances)
  *
  * Note this is not a full paxos implementation as it assumes success
  * (since it's designed for benchmarking, and failures in Paxos would be
  * much harder/inconsistent to bench)
  * Also it runs everything locally, by putting each "machine" on a different
  * port.
  *
  */

package main

import (
    "fmt"
    "net"
    "net/rpc"
    "flag"
    "math/rand"
    "log"
    "io/ioutil"
    "time"
    "os"
)

const (
    F = 5
    NACCEPTORS = 2*F + 1
    PORTBASE = 9000
    MAX_ITER = 10000
    DEBUG = false
)

// port number acceptor is running on
var portNumber int

// array of client connections (the proposer's connections to acceptors)
var clients [NACCEPTORS]*rpc.Client

var startIter int = -1

type Value int64

type Number struct {
    IterN int
    PropN int
}

// used as argument/return for RPCs
type NV struct {
    N Number
    V *Value // proposal Value
}

type AcceptorState struct {
    // these would be persistent in a full implementation
    N_l Number
    N_a Number
    V_a *Value
}

var astate AcceptorState

type ProposerState struct {
    N_p Number // persistent
    A int
    N_o Number
    V_o *Value
}

var pstate ProposerState


// RPCs!!
type Acceptor struct{}
func (t *Acceptor) Prepare(n *Number, reply *NV) error {
    log.Printf("%d got prepare message\n", portNumber)
    // if the proposer is onto a higher iteration than us, we reset our state
    // (real implementation would probably actually store decided values somewhere)
    if n.IterN > astate.N_l.IterN {
        astate.reset(n.IterN)
    }
    astate.N_l = maxNumber(astate.N_l, *n);
    log.Printf("iter: %d\n", astate.N_l.IterN)
    reply.N = astate.N_a;
    reply.V = astate.V_a;

    return nil;
}

func (t *Acceptor) Accept(nv *NV, reply *Number) error {
    if !nv.N.Less(astate.N_l) {
        astate.N_l = nv.N;
        astate.N_a = nv.N;
        astate.V_a = nv.V;
    }
    *reply = astate.N_a;

    return nil;
}

func (t *Acceptor) Decided(v *Value, reply *int) error {
    log.Printf("%d decided: %d\n", portNumber, *v);
    return nil;
}

func (n Number) Less(n2 Number) bool {
    return n.IterN < n2.IterN || (n.IterN == n2.IterN && n.PropN < n2.PropN);
}

func maxNumber(n1, n2 Number) Number {
    if n1.Less(n2) {
        return n2
    }
    return n1
}

func (a AcceptorState) reset(newIter int) {
    a.V_a = nil;
    a.N_a = Number{newIter, 0};
    a.N_l = Number{newIter, 0};
}

func acceptorInit() net.Listener {
    rpc.Register(new(Acceptor))
    ln, err := net.Listen("tcp", fmt.Sprintf(":%d", portNumber))
    if err != nil {
        fmt.Println(err)
        return nil
    }
    return ln
}

func acceptorRun(ln net.Listener) {
    for {
        c, err := ln.Accept()
        if err != nil {
            continue
        }
        rpc.ServeConn(c)
    }
}

func proposerInit() {
    rand.Seed(time.Now().Unix())
    // we assume clients are connected on sequential ports, starting at PORTBASE
    for i := 0; i < NACCEPTORS; i++ {
        p := PORTBASE + i
        c, err := rpc.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", p))
        if err != nil {
            fmt.Println(err)
            return
        }
        clients[i] = c
    }
}

func chooseVal() *Value {
    v := new(Value)
    *v = Value(rand.Int63())
    log.Printf("chose value: %d\n", *v)
    return v
}

func accepted(n *Number) bool {
    if *n == pstate.N_p {
        pstate.A = pstate.A + 1
    }
    if pstate.A == F+1 {
        log.Printf("proposer decided: %d\n", *pstate.V_o);
        // a full implementation would probably want to resend these
        // for clients that didn't respond
        for i := 0; i < NACCEPTORS; i++ {
            clients[i].Go("Acceptor.Decided", pstate.V_o, nil, nil);
        }
        // start next iteration
        pstate.N_p.IterN++
        if (pstate.N_p.IterN - startIter) >= MAX_ITER {
            os.Exit(0)
        }
        go propose()
        return true
    }
    return false
}

func sendAccepts() {
    nv := NV{pstate.N_p, pstate.V_o}
    constructor := func() interface{} { return new(Number) }
    handler := func(reply interface{}) bool { return accepted(reply.(*Number)) }
    sendAndRecv("Acceptor.Accept", &nv, constructor, handler)
}

func prepared(nv *NV) bool {
    if pstate.N_o.Less(nv.N) {
        log.Printf("going with: %p\n", nv.V)
        pstate.N_o = nv.N
        pstate.V_o = nv.V
    }

    pstate.A++

    if pstate.A == F+1 {
        if pstate.V_o == nil {
            pstate.V_o = chooseVal()
        }
        pstate.A = 0
        // so we can run MAX_ITER iterations even if we start at a nonzero
        // iteration number 
        if startIter == -1 {
            startIter = pstate.N_o.IterN
        }
        pstate.N_p = maxNumber(pstate.N_p, pstate.N_o)
        go sendAccepts()
        return true
    }
    return false
}

func sendAndRecv(msg string, args interface{}, newReply func()(interface{}), handler func(reply interface{})(bool)) {
    var calls []*rpc.Call
    for i := 0; i < F+1; i++ {
        call := clients[i].Go(msg, args, newReply(), nil)
        calls = append(calls, call)
    }

    for _, call := range calls {
        <-call.Done
        if call.Error != nil {
            log.Fatal("message error")
        }
        if handler(call.Reply) {
            break
        }
    }

/*
    // this method avoids blocking on any specific acceptor, and instead
    // spins until it finds an acceptor that has responded. it's only
    // marginally faster though so likely not worth it
    ncalls := len(calls)

    for i := 0; true; i = (i+1) % ncalls {
        select {
        case <- calls[i].Done:
            if calls[i].Error != nil {
                log.Fatal("message error");
            }
            reply := calls[i].Reply
            if handler(reply) {
                break
            }
        default:
        }
    }
*/

}

func propose() {
    log.Printf("starting proposal\n")
    // only 1 proposer so no uniqueifier
    pstate.N_p.PropN = pstate.N_p.PropN + 1
    pstate.A = 0
    pstate.N_o = Number{pstate.N_p.IterN, 0}
    pstate.V_o = nil
    
    constructor := func() interface{} { return new(NV) }
    handler := func (reply interface{}) bool { return prepared(reply.(*NV)) }

    sendAndRecv("Acceptor.Prepare", &pstate.N_p, constructor, handler)
}

func main() {
    if !DEBUG {
        // disables debug logging
        log.SetOutput(ioutil.Discard)
    }

    boolP := flag.Bool("prop", false, "run as proposer")
    portP := flag.Int("p", 9000, "port number")
    flag.Parse()

    portNumber = *portP

    ln := acceptorInit()
    if ln == nil {
        os.Exit(1)
    }

    if *boolP {
        go acceptorRun(ln)
        proposerInit()
        go propose()
        // don't exit
        <-make(chan int)
    } else {
        acceptorRun(ln)
    }
}