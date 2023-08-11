package main

import (
	"context"
	"fmt"
	"github.com/Digital-Maritime-Consultancy/mms-agent-go/generated/mmtp"
	"github.com/Digital-Maritime-Consultancy/mms-agent-go/mms"
	"os"
	"os/signal"
	"time"
)

func connectWithHandling(ctx context.Context, a *mms.Agent, url string) {
	res, err := a.Connect(ctx, url)
	if err != nil {
		fmt.Errorf("could not connect to edge router: %w", err)
	}
	if res == mmtp.ResponseEnum_GOOD {
		fmt.Println(a.Mrn, " is connected")
	}
}

func sendTextWithHandling(ctx context.Context, a *mms.Agent, receivingMrn string, msg string) {
	res, err := a.Send(ctx, time.Duration(10), receivingMrn, []byte(msg))
	if err != nil {
		fmt.Errorf("could not send to edge router: %w", err)
	}
	if res == mmtp.ResponseEnum_GOOD {
		fmt.Println(a.Mrn, "--[", msg, ", to ", receivingMrn, "]--> MMS")
	}
}

func sendDataWithHandling(ctx context.Context, a *mms.Agent, receivingMrn string, data []byte) {
	res, err := a.Send(ctx, time.Duration(10), receivingMrn, data)
	if err != nil {
		fmt.Errorf("could not send to edge router: %w", err)
	}
	if res == mmtp.ResponseEnum_GOOD {
		fmt.Println(a.Mrn, "--[", "data", ", to ", receivingMrn, "]--> MMS")
	}
}

func receiveWithHandling(ctx context.Context, a *mms.Agent) []string {
	var msgList []string
	_, msgList, err := a.Receive(ctx)
	if err != nil {
		fmt.Errorf("could not send to edge router: %w", err)
	}
	fmt.Println("MMS --[", msgList, "]--> ", a.Mrn)
	return msgList
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	// wait for a SIGINT or SIGTERM signal
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)

	const url = "localhost:8080"
	var agentMrn1 = "urn:mrn:mcp:device:idp1:org1:agent1"
	var agentMrn2 = "urn:mrn:mcp:device:idp1:org1:agent2"

	// Read the file into a slice of bytes
	data, err := os.ReadFile("data/S411_20230504_092247_back_to_20230430_095254_Greenland_ASIP.gml")
	//data, err := os.ReadFile("data/test.txt")
	if err != nil {
		// Handle the error
		fmt.Errorf("there was an error in reading file")
		return
	}

	agent1 := mms.NewAgent(agentMrn1)
	agent2 := mms.NewAgent(agentMrn2)

	connectWithHandling(ctx, agent1, url)
	connectWithHandling(ctx, agent2, url)

	sendDataWithHandling(ctx, agent1, agent2.Mrn, data)

	time.Sleep(time.Second)

	fmt.Println(agent2.Mrn, ":", receiveWithHandling(ctx, agent2))

	<-ch
	fmt.Println("Received signal, shutting down...")
	cancel()
}
