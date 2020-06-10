package main

import (
	"bitcoin"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
)

func main() {
	const numArgs = 4
	if len(os.Args) != numArgs {
		fmt.Printf("Usage: ./%s <hostport> <message> <maxNonce>", os.Args[0])
		return
	}
	hostport := os.Args[1]
	message := os.Args[2]
	maxNonce, err := strconv.ParseUint(os.Args[3], 10, 64)
	if err != nil {
		fmt.Printf("%s is not a number.\n", os.Args[3])
		return
	}

	client, err := net.Dial("tcp", hostport)
	if err != nil {
		fmt.Println("Failed to connect to server:", err)
		return
	}

	defer client.Close()
	// TODO: implement this!
	// Make new job with request from parsed values
	job := bitcoin.NewRequest(message, 0, maxNonce)
	// marshal job
	jsonJob, _ := json.Marshal(job)
	// send job to server(named client)
	client.Write(jsonJob)
	// prepare to recieve reply from server
	msg := make([]byte, 256)
	// recieve reply from server and read it
	n, err := client.Read(msg)
	if err != nil {
		printDisconnected()
		return
	}
	// ready to unmarshal reply
	var mesg bitcoin.Message
	// n bytes read hence msg only read till there
	err = json.Unmarshal(msg[:n], &mesg)
	// print the result
	printResult(mesg.Hash, mesg.Nonce)
	return

}

// printResult prints the final result to stdout.
func printResult(hash, nonce uint64) {
	fmt.Println("Result", hash, nonce)
}

// printDisconnected prints a disconnected message to stdout.
func printDisconnected() {
	fmt.Println("Disconnected")
}
