package main

import (
	"bitcoin"
	"encoding/json"
	"fmt"
	"net"
	"os"
)

// Attempt to connect miner as a client to the server.
func joinWithServer(hostport string) (net.Conn, error) {
	conn, err := net.Dial("tcp", hostport)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func main() {
	const numArgs = 2
	if len(os.Args) != numArgs {
		fmt.Printf("Usage: ./%s <hostport>", os.Args[0])
		return
	}

	hostport := os.Args[1]
	conn, err := joinWithServer(hostport)

	if err != nil {
		return
	}

	defer conn.Close()
	// make new join request to let server know that it's miner
	joinReq := bitcoin.NewJoin()
	// marshalling the request
	jsonReq, _ := json.Marshal(joinReq)
	// sending it
	conn.Write(jsonReq)
	msg := make([]byte, 256)
	// serve the server until it dies or error
	for {
		// receive the request from server
		n, err := conn.Read(msg)
		if err != nil {
			return
		}
		var mesg bitcoin.Message
		// unmarshall it
		err = json.Unmarshal(msg[:n], &mesg)
		// if server crashes return
		if err != nil {
			return
		}
		// get all the relevant data from received file
		data := mesg.Data
		minNonce := mesg.Lower
		maxNonce := mesg.Upper
		// initiliazing lowest nonce to first
		lowestNonce := uint64(0)
		// initiliazing min hash to highest uint
		minHash := ^uint64(0)
		// check hashing for all values in the range and returning lowest hash and corresponding nonce
		for i := minNonce; i < maxNonce; i++ {
			hash := bitcoin.Hash(data, i)
			if hash < minHash {
				minHash = hash
				lowestNonce = i
			}
		}
		// make result
		result := bitcoin.NewResult(minHash, uint64(lowestNonce))
		// marshall it
		jsonResult, _ := json.Marshal(result)
		// send back
		conn.Write(jsonResult)

	}

}
