package mms

import (
	"context"
	"fmt"
	"github.com/maritimeconnectivity/mms-agent-go/generated/mmtp"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

func ConnectWithHandling(ctx context.Context, a *Agent, url string) error {
	res, err := a.ConnectAuthenticated(ctx, url)
	if err != nil || res != mmtp.ResponseEnum_GOOD {
		fmt.Println("could not connect to edge router: %w", err)
		return err
	}
	fmt.Println(a.Mrn, "is connected")
	return nil
}

func ConnectAnonymousWithHandling(ctx context.Context, a *Agent, url string) error {
	res, err := a.ConnectAnonymous(ctx, url)
	if err != nil || res != mmtp.ResponseEnum_GOOD {
		fmt.Println("could not connect to edge router: %w", err)
		return err
	}
	fmt.Println(a.Mrn, "is connected anonymously")
	return nil
}

func DisconnectWithHandling(ctx context.Context, a *Agent) {
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

func SendTextWithHandling(ctx context.Context, a *Agent, receivingMrn string, msg string) {
	res, err := a.Send(ctx, time.Duration(10), receivingMrn, []byte(msg))
	if err != nil {
		fmt.Println("could not send to edge router: %w", err)
	}
	if res == mmtp.ResponseEnum_GOOD {
		fmt.Println(a.Mrn, "--[", msg, ", to ", receivingMrn, "]--> MMS")
	}
}

func EncodeFileName(fileName string, data []byte) []byte {
	return append([]byte("FILE"+filepath.Base(fileName)+"FILE"), data...)
}

func DecodeFileName(data []byte) (string, []byte) {
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

func SendDataWithHandling(ctx context.Context, a *Agent, receivingMrn string, data []byte) {
	res, err := a.Send(ctx, time.Duration(10), receivingMrn, data)
	if err != nil {
		fmt.Println("could not send to edge router: %w", err)
	}
	if res == mmtp.ResponseEnum_GOOD {
		fmt.Println(a.Mrn, "--send[", "data", ", to ", receivingMrn, "]--> MMS")
	}
}

func PublishDataWithHandling(ctx context.Context, a *Agent, subject string, data []byte) {
	res, err := a.Publish(ctx, time.Duration(10), subject, data)
	if err != nil {
		fmt.Println("could not publish data: %w", err)
	}
	if res == mmtp.ResponseEnum_GOOD {
		fmt.Println(a.Mrn, "--publish[", "data", ", to subject ", subject, "]--> MMS")
	}
}

func ReceiveWithHandling(ctx context.Context, a *Agent) [][]byte {
	var msgList [][]byte
	_, msgList, err := a.Receive(ctx, nil)
	if err != nil {
		fmt.Println("could not receive from edge router: %w", err)
	}
	fmt.Println("MMS --", len(msgList), "messages -->", a.Mrn, "\n")
	for idx, msg := range msgList {
		fileName, data := DecodeFileName(msg[:])
		if len(fileName) > 0 {
			fmt.Println(idx, "-", fileName, ":", string(data))
		} else {
			fmt.Println(idx, ":", string(msg[:]))
		}
	}
	return msgList
}

// readFile reads the file into a slice of bytes
func ReadFile(ctx context.Context, dataFileName string) ([]byte, error) {
	data, err := os.ReadFile(dataFileName)
	if err != nil {
		return nil, err
	}

	// append a fileName with specific format
	data = EncodeFileName(dataFileName, data)

	return data, nil
}

func SendDataOverDirectMessage(ctx context.Context, sender *Agent, receiver *Agent, dataFileName string) {
	data, err := ReadFile(ctx, dataFileName)
	if err != nil {
		// Handle the error
		log.Fatalln("there was an error in reading file ", dataFileName, ":", err)
		return
	}

	SendDataWithHandling(ctx, sender, receiver.Mrn, data)

	time.Sleep(time.Second)

	ReceiveWithHandling(ctx, receiver)
	fmt.Println("test done - sendDataOverDirectMessage")
}

func SubscribeTopic(ctx context.Context, sender *Agent, receiver *Agent, subject string) {
	receiver.Subscribe(ctx, subject)

	PublishDataWithHandling(ctx, sender, subject, []byte("Hello1"))
	time.Sleep(time.Second)

	ReceiveWithHandling(ctx, receiver)

	PublishDataWithHandling(ctx, sender, subject, []byte("Hello2"))
	PublishDataWithHandling(ctx, sender, subject, []byte("Hello3"))
	time.Sleep(time.Second * 10)

	ReceiveWithHandling(ctx, receiver)
	time.Sleep(time.Second)

	ReceiveWithHandling(ctx, receiver)
	fmt.Println("test done - subscribeTopic")
}

func SubAndUnsubscribeTopic(ctx context.Context, sender *Agent, receiver *Agent, subject string) {
	receiver.Subscribe(ctx, subject)

	PublishDataWithHandling(ctx, sender, subject, []byte("Hello1"))
	time.Sleep(time.Second)

	ReceiveWithHandling(ctx, receiver)

	time.Sleep(time.Second)
	receiver.Unsubscribe(ctx, subject)

	PublishDataWithHandling(ctx, sender, subject, []byte("Hello2"))
	PublishDataWithHandling(ctx, sender, subject, []byte("Hello3"))
	time.Sleep(time.Second)

	ReceiveWithHandling(ctx, receiver)
	fmt.Println("test done - subAndUnsubscribeTopic")
}

func SubAndUnsubscribeWithData(ctx context.Context, sender *Agent, receiver *Agent, subject string, dataFileName string) {
	data, err := ReadFile(ctx, dataFileName)
	if err != nil {
		// Handle the error
		log.Fatalln("there was an error in reading file ", dataFileName, ":", err)
		return
	}

	receiver.Subscribe(ctx, subject)

	PublishDataWithHandling(ctx, sender, subject, data)
	time.Sleep(time.Second)

	ReceiveWithHandling(ctx, receiver)

	time.Sleep(time.Second)
	receiver.Unsubscribe(ctx, subject)

	PublishDataWithHandling(ctx, sender, subject, []byte("Hello2"))
	PublishDataWithHandling(ctx, sender, subject, []byte("Hello3"))
	time.Sleep(time.Second)

	ReceiveWithHandling(ctx, receiver)
	fmt.Println("test done - subAndUnsubscribeTopic")
}

func SubUnsubReconnection(ctx context.Context, sender *Agent, receiver *Agent, subject string, url string) {
	receiver.Subscribe(ctx, subject)

	PublishDataWithHandling(ctx, sender, subject, []byte("Hello1"))
	time.Sleep(time.Second)

	ReceiveWithHandling(ctx, receiver)

	time.Sleep(time.Second)
	receiver.Disconnect(ctx)

	PublishDataWithHandling(ctx, sender, subject, []byte("Hello2"))
	PublishDataWithHandling(ctx, sender, subject, []byte("Hello3"))
	time.Sleep(time.Second)

	receiver.ConnectAuthenticated(ctx, url)
	time.Sleep(time.Second)

	ReceiveWithHandling(ctx, receiver)
	fmt.Println("test done - subUnsubReconnection")
}

func ConnectAndSubAnonymous(ctx context.Context, sender *Agent, receiverAuthenticated *Agent, url string, topic string, dataFileName string) {
	data, err := ReadFile(ctx, dataFileName)
	if err != nil {
		// Handle the error
		log.Fatalln("there was an error in reading file ", dataFileName, ":", err)
		return
	}

	receiverAnonymous := NewAgent("")

	ConnectAnonymousWithHandling(ctx, receiverAnonymous, url)

	receiverAnonymous.Subscribe(ctx, topic)

	receiverAuthenticated.Subscribe(ctx, topic)

	PublishDataWithHandling(ctx, sender, topic, data)
	time.Sleep(time.Second)

	ReceiveWithHandling(ctx, receiverAnonymous)
	ReceiveWithHandling(ctx, receiverAuthenticated)

	time.Sleep(time.Second)
	ReceiveWithHandling(ctx, receiverAnonymous)
	ReceiveWithHandling(ctx, receiverAuthenticated)
	DisconnectWithHandling(ctx, receiverAnonymous)
	fmt.Println("test done - connectAndSubAnonymous")
}
