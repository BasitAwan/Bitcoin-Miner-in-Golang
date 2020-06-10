package main

import (
	"bitcoin"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
)

var clientsMinNonce map[net.Conn]uint64 //map to keep track of minNonce of each client
var clientsMinHash map[net.Conn]uint64  //map to keep track of minhash of each client
var clientNum map[net.Conn]*int64       // map to keep track how many jobs left
type server struct {
	listener net.Listener
}

type clientRequest struct { // struct made to communicate between go routines, as to know which request belongs to which client
	conn    net.Conn         // conn type to differentiate between clients
	request *bitcoin.Message // message type to store requests
}

func startServer(port int) (*server, error) {
	clientsMinNonce = make(map[net.Conn]uint64)
	clientsMinHash = make(map[net.Conn]uint64)
	clientNum = make(map[net.Conn]*int64)
	port1 := ":" + strconv.Itoa(port)
	ln, err := net.Listen("tcp", port1)
	send := make(chan clientRequest)               // receive all the work done by all miners
	handler := make(chan clientRequest)            // recieves initial request made by client
	toMiners := make(chan clientRequest, 100)      // used to receive smaller requests
	toMinersPrior := make(chan clientRequest, 100) // prioritized channel for smaller requests
	for index := 0; index < 5; index++ {
		// function to process client requests
		go handleClient(handler, toMiners, toMinersPrior)

	}
	go resultCompiler(send) // function to recieve results and then compile them
	if err != nil {
		return nil, err
	}
	for {
		conn, err := ln.Accept()

		if err != nil {

		} else {
			go handleConnection(conn, send, toMinersPrior, toMiners, handler) // function to decide between miner and client
		}

	}
}

// takes big client request splits it into pieces of 10000 and sends to different miners
func handleClient(handler, toMiners, toMinersPrior chan clientRequest) {
	for {
		clientReq := <-handler
		request := clientReq.request
		maxNonce := request.Upper
		// needed to use pointer to change values in map
		temp := new(int64)
		*temp = int64(maxNonce)
		clientNum[clientReq.conn] = temp
		toSend := toMiners //default channel to send to
		if maxNonce < 100000 {
			toSend = toMinersPrior // in case small request
		}
		// variable used to signal to ask for miner
		for i := 0; i < (int(maxNonce) / 10000); i++ {
			//makes new request with 10000 nonces
			req := bitcoin.NewRequest(request.Data, uint64(i*10000), uint64(i*10000+10000))
			// makes new request to send to compiler
			var job clientRequest
			job.request = req
			job.conn = clientReq.conn
			// sends job to miner
			toSend <- job
		}
		// used to treat the missing last portion
		req := bitcoin.NewRequest(request.Data, uint64((maxNonce/10000)*10000), uint64(maxNonce))
		var job clientRequest
		job.request = req
		job.conn = clientReq.conn
		toSend <- job
	}
}

// gets all results from miners and works out all the results for all clients
func resultCompiler(send chan clientRequest) {
	for {
		// recieved worked result
		clientReq := <-send
		result := clientReq.request
		// checks if if new hash is less already stored hash
		if clientsMinHash[clientReq.conn] > result.Hash {
			clientsMinHash[clientReq.conn] = result.Hash
			clientsMinNonce[clientReq.conn] = result.Nonce
		}
		// used to update number of jobs left
		temp := clientNum[clientReq.conn]
		*temp = *temp - 10000
		// Checks if total number is over
		if *clientNum[clientReq.conn] < 0 {
			// makes result with the present values
			finalResult := bitcoin.NewResult(clientsMinHash[clientReq.conn], clientsMinNonce[clientReq.conn])
			jsonResult, _ := json.Marshal(finalResult)
			// sends to client
			(clientReq.conn).Write(jsonResult)
		}
	}
}

// function to decide between miner and client
func handleConnection(conn net.Conn, send, toMinersPrior, toMiners, handler chan clientRequest) {
	msg := make([]byte, 256)
	n, err := conn.Read(msg)
	// read message and use n to know how much read
	var mesg *bitcoin.Message
	err = json.Unmarshal(msg[:n], &mesg)

	if err != nil {
		return
	}

	if mesg.Type == bitcoin.Join {
		// go to miner function with the same send and receive chan
		miner(conn, send, toMinersPrior, toMiners)
	} else { //client request
		clientsMinHash[conn] = ^uint64(0) // initiliazing min hash for particular client to max uint
		clientsMinNonce[conn] = 0         // initiliazing minNonce to first
		var newClient clientRequest       // make new client request and fill it up
		newClient.conn = conn
		newClient.request = mesg
		handler <- newClient // sending the request to one of the handler

	}

}

func miner(conn net.Conn, send, toMinersPrior, toMiners chan clientRequest) {
	// function to handle each client, recieves reqeuests from handleClient and sends to result compiler function
	for {
		// receives request
		select {
		// the case-default-continue (as I like to call it), prioritizes the toMinersPrior channel over other
		case clientReq := <-toMinersPrior:
			okay := processReq(conn, send, toMinersPrior, toMiners, clientReq)
			if okay {
				continue
			} else {
				return
			}
		default:
			select {
			case clientReq := <-toMiners:
				okay := processReq(conn, send, toMinersPrior, toMiners, clientReq)
				if okay {
					continue
				} else {
					return
				}
			default:
				continue

			}

		}

	}
}

func processReq(conn net.Conn, send, toMinersPrior, toMiners chan clientRequest, clientReq clientRequest) bool {
	request := clientReq.request
	job, err := json.Marshal(request)
	if err != nil {
	}
	_, err = conn.Write(job)
	if err != nil {
		toMinersPrior <- clientReq
		return false

	}
	msg := make([]byte, 4096)
	// standard recieving and unmarshalling
	n, err := io.ReadAtLeast(conn, msg, 24)
	if err != nil {
		toMinersPrior <- clientReq
		return false

	}

	mesg := new(bitcoin.Message)
	err = json.Unmarshal(msg[:n], mesg)
	if err != nil {
	}
	// making new clientRequest to send to compiler
	var result clientRequest
	result.request = mesg
	result.conn = clientReq.conn
	send <- result

	return true
}

func main() {

	const numArgs = 2
	if len(os.Args) != numArgs {
		fmt.Printf("Usage: ./%s <port>", os.Args[0])
		return
	}
	port, err := strconv.Atoi(os.Args[1])
	if err != nil {
		fmt.Println("Port must be a number:", err)
		return
	}
	srv, err := startServer(port)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	fmt.Println("Server listening on port", port)
	defer srv.listener.Close()

}
