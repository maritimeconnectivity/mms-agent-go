package main

import (
	"context"
	"fmt"
	"github.com/maritimeconnectivity/mms-agent-go/generated/mmtp"
	"github.com/maritimeconnectivity/mms-agent-go/mms"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

func connectWithHandling(ctx context.Context, a *mms.Agent, url string) error {
	res, err := a.ConnectAuthenticated(ctx, url)
	if err != nil || res != mmtp.ResponseEnum_GOOD {
		fmt.Println("could not connect to edge router: %w", err)
		return err
	}
	fmt.Println(a.Mrn, "is connected")
	return nil
}

func connectAnonymousWithHandling(ctx context.Context, a *mms.Agent, url string) error {
	res, err := a.ConnectAnonymous(ctx, url)
	if err != nil || res != mmtp.ResponseEnum_GOOD {
		fmt.Println("could not connect to edge router: %w", err)
		return err
	}
	fmt.Println(a.Mrn, "is connected anonymously")
	return nil
}

func disconnectWithHandling(ctx context.Context, a *mms.Agent) {
	res, err := a.Disconnect(ctx)
	if err != nil {
		fmt.Println("could not disconnect from edge router: %w", err)
	}
	if res == mmtp.ResponseEnum_GOOD {
		fmt.Println(a.Mrn, "is disconnected")
	} else {
		fmt.Println(a.Mrn, "disconnection failed: %w", err)
	}
}

func sendTextWithHandling(ctx context.Context, a *mms.Agent, receivingMrn string, msg string) {
	res, err := a.Send(ctx, time.Duration(10), receivingMrn, []byte(msg))
	if err != nil {
		fmt.Println("could not send to edge router: %w", err)
	}
	if res == mmtp.ResponseEnum_GOOD {
		fmt.Println(a.Mrn, "--[", msg, ", to ", receivingMrn, "]--> MMS")
	}
}

func encodeFileName(fileName string, data []byte) []byte {
	return append([]byte("FILE"+filepath.Base(fileName)+"FILE"), data...)
}

func regSplit(text string, delimeter string) []string {
	regex := regexp.MustCompile(delimeter)
	return regex.Split(text, -1)
}
func decodeFileName(data []byte) (string, []byte) {
	match, _ := regexp.MatchString("FILE", string(data))
	if match {
		var filtered []string
		dataStr := strings.Split(string(data), "FILE")
		for _, data := range dataStr {
			if len(data) > 0 {
				filtered = append(filtered, data)
			}
		}
		return filtered[0], []byte(filtered[1])
	}
	return "", data
}

func sendDataWithHandling(ctx context.Context, a *mms.Agent, receivingMrn string, data []byte) {
	res, err := a.Send(ctx, time.Duration(10), receivingMrn, data)
	if err != nil {
		fmt.Println("could not send to edge router: %w", err)
	}
	if res == mmtp.ResponseEnum_GOOD {
		fmt.Println(a.Mrn, "--send[", "data", ", to ", receivingMrn, "]--> MMS")
	}
}

func publishDataWithHandling(ctx context.Context, a *mms.Agent, subject string, data []byte) {
	res, err := a.Publish(ctx, time.Duration(10), subject, data)
	if err != nil {
		fmt.Println("could not publish data: %w", err)
	}
	if res == mmtp.ResponseEnum_GOOD {
		fmt.Println(a.Mrn, "--publish[", "data", ", to subject ", subject, "]--> MMS")
	}
}

func receiveWithHandling(ctx context.Context, a *mms.Agent) [][]byte {
	var msgList [][]byte
	_, msgList, err := a.Receive(ctx, nil)
	if err != nil {
		fmt.Println("could not receive from edge router: %w", err)
	}
	fmt.Println("MMS --", len(msgList), "messages -->", a.Mrn, "\n")
	for idx, msg := range msgList {
		fileName, data := decodeFileName(msg[:])
		if len(fileName) > 0 {
			fmt.Println(idx, "-", fileName, ":", string(data))
		} else {
			fmt.Println(idx, ":", string(msg[:]))
		}
	}
	return msgList
}

// readFile reads the file into a slice of bytes
func readFile(ctx context.Context, dataFileName string) ([]byte, error) {
	data, err := os.ReadFile(dataFileName)
	if err != nil {
		return nil, err
	}

	// append a fileName with specific format
	data = encodeFileName(dataFileName, data)

	return data, nil
}

func sendDataOverDirectMessage(ctx context.Context, sender *mms.Agent, receiver *mms.Agent, dataFileName string) {
	data, err := readFile(ctx, dataFileName)
	if err != nil {
		// Handle the error
		log.Fatalln("there was an error in reading file ", dataFileName, ":", err)
		return
	}

	sendDataWithHandling(ctx, sender, receiver.Mrn, data)

	time.Sleep(time.Second)

	receiveWithHandling(ctx, receiver)
	fmt.Println("test done - sendDataOverDirectMessage")
}

func subscribeTopic(ctx context.Context, sender *mms.Agent, receiver *mms.Agent, subject string) {
	receiver.Subscribe(ctx, subject)

	publishDataWithHandling(ctx, sender, subject, []byte("Hello1"))
	time.Sleep(time.Second)

	receiveWithHandling(ctx, receiver)

	publishDataWithHandling(ctx, sender, subject, []byte("Hello2"))
	publishDataWithHandling(ctx, sender, subject, []byte("Hello3"))
	time.Sleep(time.Second * 10)

	receiveWithHandling(ctx, receiver)
	time.Sleep(time.Second)

	receiveWithHandling(ctx, receiver)
	fmt.Println("test done - subscribeTopic")
}

func subAndUnsubscribeTopic(ctx context.Context, sender *mms.Agent, receiver *mms.Agent, subject string) {
	receiver.Subscribe(ctx, subject)

	publishDataWithHandling(ctx, sender, subject, []byte("Hello1"))
	time.Sleep(time.Second)

	receiveWithHandling(ctx, receiver)

	time.Sleep(time.Second)
	receiver.Unsubscribe(ctx, subject)

	publishDataWithHandling(ctx, sender, subject, []byte("Hello2"))
	publishDataWithHandling(ctx, sender, subject, []byte("Hello3"))
	time.Sleep(time.Second)

	receiveWithHandling(ctx, receiver)
	fmt.Println("test done - subAndUnsubscribeTopic")
}

func subAndUnsubscribeWithData(ctx context.Context, sender *mms.Agent, receiver *mms.Agent, subject string, dataFileName string) {
	data, err := readFile(ctx, dataFileName)
	if err != nil {
		// Handle the error
		log.Fatalln("there was an error in reading file ", dataFileName, ":", err)
		return
	}

	receiver.Subscribe(ctx, subject)

	publishDataWithHandling(ctx, sender, subject, data)
	time.Sleep(time.Second)

	receiveWithHandling(ctx, receiver)

	time.Sleep(time.Second)
	receiver.Unsubscribe(ctx, subject)

	publishDataWithHandling(ctx, sender, subject, []byte("Hello2"))
	publishDataWithHandling(ctx, sender, subject, []byte("Hello3"))
	time.Sleep(time.Second)

	receiveWithHandling(ctx, receiver)
	fmt.Println("test done - subAndUnsubscribeTopic")
}

func subUnsubReconnection(ctx context.Context, sender *mms.Agent, receiver *mms.Agent, subject string, url string) {
	receiver.Subscribe(ctx, subject)

	publishDataWithHandling(ctx, sender, subject, []byte("Hello1"))
	time.Sleep(time.Second)

	receiveWithHandling(ctx, receiver)

	time.Sleep(time.Second)
	receiver.Disconnect(ctx)

	publishDataWithHandling(ctx, sender, subject, []byte("Hello2"))
	publishDataWithHandling(ctx, sender, subject, []byte("Hello3"))
	time.Sleep(time.Second)

	receiver.ConnectAuthenticated(ctx, url)
	time.Sleep(time.Second)

	receiveWithHandling(ctx, receiver)
	fmt.Println("test done - subUnsubReconnection")
}

func connectAndSubAnonymous(ctx context.Context, sender *mms.Agent, receiverAuthenticated *mms.Agent, url string, dataFileName string) {
	data, err := readFile(ctx, dataFileName)
	if err != nil {
		// Handle the error
		log.Fatalln("there was an error in reading file ", dataFileName, ":", err)
		return
	}

	const subject = "anonymousTest"

	receiverAnonymous := mms.NewAgent("")

	connectAnonymousWithHandling(ctx, receiverAnonymous, url)

	receiverAnonymous.Subscribe(ctx, subject)

	receiverAuthenticated.Subscribe(ctx, subject)

	publishDataWithHandling(ctx, sender, subject, data)
	time.Sleep(time.Second)

	receiveWithHandling(ctx, receiverAnonymous)
	receiveWithHandling(ctx, receiverAuthenticated)

	time.Sleep(time.Second)
	receiveWithHandling(ctx, receiverAnonymous)
	receiveWithHandling(ctx, receiverAuthenticated)
	disconnectWithHandling(ctx, receiverAnonymous)
	fmt.Println("test done - connectAndSubAnonymous")
}

func doIntegrationTest(ctx context.Context) {
	const url = "localhost:8888"
	var agentMrn1 = "urn:mrn:mcp:device:idp1:org1:agent1"
	var agentMrn2 = "urn:mrn:mcp:device:idp1:org1:agent2"

	agent1 := mms.NewAgent(agentMrn1)
	agent2 := mms.NewAgent(agentMrn2)

	err1 := connectWithHandling(ctx, agent1, url)
	if err1 != nil {
		fmt.Println("could not connect to edge router: %w", err1)
		return
	}

	err2 := connectWithHandling(ctx, agent2, url)
	if err2 != nil {
		fmt.Println("could not connect to edge router: %w", err2)
		return
	}

	const dataFileName = "data/test.txt" // "data/S411_20230504_092247_back_to_20230430_095254_Greenland_ASIP.gml"

	// integration tests:
	// test case 1 - direct message from one to another
	//sendDataOverDirectMessage(ctx, agent1, agent2, dataFileName)

	// test case 2 - subscribed message from topic
	//subscribeTopic(ctx, agent1, agent2, "test")

	// test case 3 - subscription and unsubscription
	//subAndUnsubscribeTopic(ctx, agent1, agent2, "test")

	//subAndUnsubscribeWithData(ctx, agent1, agent2, "test", dataFileName)

	// TODO: test case 4 - subscription and unsubscription with reconnection
	//subUnsubReconnection(ctx, agent1, agent2, "test", url)

	// test case 5 - subscription with an anonymous user
	connectAndSubAnonymous(ctx, agent1, agent2, url, dataFileName)

	disconnectWithHandling(ctx, agent1)
	disconnectWithHandling(ctx, agent2)
}

func testFileNameEncoding(ctx context.Context) {
	const dataFileName = "data/test.txt" // "data/S411_20230504_092247_back_to_20230430_095254_Greenland_ASIP.gml"
	data, _ := readFile(ctx, dataFileName)
	fmt.Println(string(data))
	name, data := decodeFileName(data)
	if name == dataFileName {
		fmt.Println(name, data)
	}
	name, data = decodeFileName([]byte("Teest"))
	fmt.Println(name, data)
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	doIntegrationTest(ctx)

	//testFileNameEncoding(ctx)

	// wait for a SIGINT or SIGTERM signal
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)

	<-ch
	fmt.Println("Received signal, shutting down...")
	cancel()
}
