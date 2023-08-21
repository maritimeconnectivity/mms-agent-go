package main

import (
	"context"
	"fmt"
	"github.com/maritimeconnectivity/mms-agent-go/mms"
	"os"
	"os/signal"
)

func doIntegrationTest(ctx context.Context) {
	const url = "localhost:8888"
	var agentMrn1 = "urn:mrn:mcp:device:idp1:org1:agent1"
	var agentMrn2 = "urn:mrn:mcp:device:idp1:org1:agent2"

	agent1 := mms.NewAgent(agentMrn1)
	agent2 := mms.NewAgent(agentMrn2)

	err1 := mms.ConnectWithHandling(ctx, agent1, url)
	if err1 != nil {
		fmt.Println("could not connect to edge router: %w", err1)
		return
	}

	err2 := mms.ConnectWithHandling(ctx, agent2, url)
	if err2 != nil {
		fmt.Println("could not connect to edge router: %w", err2)
		return
	}

	const dataFileName = "data/test.txt" // "data/S411_20230504_092247_back_to_20230430_095254_Greenland_ASIP.gml"

	// integration tests:
	// test case 1 - direct message from one to another
	mms.SendDataOverDirectMessage(ctx, agent1, agent2, dataFileName)

	// test case 2 - subscribed message from topic
	//mms.SubscribeTopic(ctx, agent1, agent2, "test")

	// test case 3 - subscription and unsubscription
	//mms.SubAndUnsubscribeTopic(ctx, agent1, agent2, "test")

	//mms.SubAndUnsubscribeWithData(ctx, agent1, agent2, "test", dataFileName)

	// TODO: test case 4 - subscription and unsubscription with reconnection
	//mms.SubUnsubReconnection(ctx, agent1, agent2, "test", url)

	// test case 5 - subscription with an anonymous user
	//mms.ConnectAndSubAnonymous(ctx, agent1, agent2, url, agentMrn2, dataFileName)

	mms.DisconnectWithHandling(ctx, agent1)
	mms.DisconnectWithHandling(ctx, agent2)
}

func testFileNameEncoding(ctx context.Context) {
	const dataFileName = "data/test.txt" // "data/S411_20230504_092247_back_to_20230430_095254_Greenland_ASIP.gml"
	data, _ := mms.ReadFile(ctx, dataFileName)
	fmt.Println(string(data))
	name, data := mms.DecodeFileName(data)
	if name == dataFileName {
		fmt.Println(name, data)
	}
	name, data = mms.DecodeFileName([]byte("Test"))
	fmt.Println(name, data)
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	// wait for a SIGINT or SIGTERM signal
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)

	// integration tests:
	doIntegrationTest(ctx)
	//testFileNameEncoding(ctx)

	<-ch
	fmt.Println("Received signal, shutting down...")
	cancel()
}
