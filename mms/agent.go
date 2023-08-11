/*
 * Copyright 2023 Maritime Connectivity Platform Consortium
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package mms

import (
	"context"
	"errors"
	"fmt"
	"github.com/Digital-Maritime-Consultancy/mms-agent-go/generated/mmtp"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"nhooyr.io/websocket"
	"sync"
	"time"
)

// Agent type representing a connected Edge Router
type Agent struct {
	Mrn            string                       // the MRN of the Agent
	Interests      []string                     // the Interests that the Agent wants to subscribe to
	Messages       map[string]*mmtp.MmtpMessage // the incoming messages for this Agent
	msgMu          *sync.RWMutex                // RWMutex for locking the Messages map
	reconnectToken string                       // token for reconnecting to a previous session
	state          AgentState                   // state of Agent
	ws             *websocket.Conn              // websocket connection bind
}

type AgentState int16

var errNotConnected = errors.New("mms is not connected to an Edge Router")

const (
	AgentState_NOTCONNECTED  AgentState = 0
	AgentState_CONNECTED     AgentState = 1
	AgentState_AUTHENTICATED AgentState = 2
)

// Status returns the the current status of the MMS Agent
func Status() []string {
	var edgeRouterMRNs []string
	fmt.Println(edgeRouterMRNs)
	return edgeRouterMRNs
}

// Discover looks up possible MMS Edge Routers
func Discover() {

}

// Connect make connect anonymously to an MMS Edge Router
func (a *Agent) Connect(ctx context.Context, url string) (mmtp.ResponseEnum, error) {
	err := errors.New("err")
	ws, err := connectWS(ctx, url)
	a.ws = ws

	if err != nil {
		fmt.Errorf("could not create web socket connection: %w", err)
	}

	res, errMmtp := a.connectOverMMTP(ctx)
	if res == nil || errMmtp != nil {
		fmt.Println("there was a problem in connection to MMS Edge Router", errMmtp)
		return mmtp.ResponseEnum_ERROR, errMmtp
	} else if res.GetResponseMessage().Response != mmtp.ResponseEnum_GOOD {
		fmt.Println("MMS Edge Router did not accept Connect:", res.GetResponseMessage().GetReasonText())
		return mmtp.ResponseEnum_ERROR, errMmtp
	}

	a.convertState(AgentState_CONNECTED)
	return res.GetResponseMessage().Response, err
}

// ReconnectAnonymous reconnect anonymously to an MMS Edge Router
func (a *Agent) ReconnectAnonymous() {

}

// Disconnect disconnects from the MMS Edge Router
func (a *Agent) Disconnect() {

}

func (a *Agent) Send(ctx context.Context, timeToLive time.Duration, receivingMrn string, content string) (mmtp.ResponseEnum, error) {
	if a.state == AgentState_NOTCONNECTED {
		return mmtp.ResponseEnum_ERROR, errNotConnected
	}

	bytes := []byte(content)
	sendMsg := &mmtp.MmtpMessage{
		MsgType: mmtp.MsgType_PROTOCOL_MESSAGE,
		Uuid:    uuid.NewString(),
		Body: &mmtp.MmtpMessage_ProtocolMessage{
			ProtocolMessage: &mmtp.ProtocolMessage{
				ProtocolMsgType: mmtp.ProtocolMessageType_SEND_MESSAGE,
				Body: &mmtp.ProtocolMessage_SendMessage{
					SendMessage: &mmtp.Send{
						ApplicationMessage: &mmtp.ApplicationMessage{
							Header: &mmtp.ApplicationMessageHeader{
								SubjectOrRecipient: &mmtp.ApplicationMessageHeader_Recipients{
									Recipients: &mmtp.Recipients{
										Recipients: []string{receivingMrn},
									},
								},
								BodySizeNumBytes: uint32(len(bytes)),
								Sender:           a.Mrn,
							},
							Body: bytes,
						},
					},
				},
			},
		},
	}
	err := writeMessage(ctx, a.ws, sendMsg)
	if err != nil {
		fmt.Println("Could not sendMsg response:", err)
		return mmtp.ResponseEnum_ERROR, err
	}
	return mmtp.ResponseEnum_GOOD, nil
}

func (a *Agent) Receive(ctx context.Context) (mmtp.ResponseEnum, []string, error) {
	if a.state == AgentState_NOTCONNECTED {
		return mmtp.ResponseEnum_ERROR, nil, errNotConnected
	}

	receiveMsg := &mmtp.MmtpMessage{
		MsgType: mmtp.MsgType_PROTOCOL_MESSAGE,
		Uuid:    uuid.NewString(),
		Body: &mmtp.MmtpMessage_ProtocolMessage{
			ProtocolMessage: &mmtp.ProtocolMessage{
				ProtocolMsgType: mmtp.ProtocolMessageType_RECEIVE_MESSAGE,
				Body: &mmtp.ProtocolMessage_ReceiveMessage{
					ReceiveMessage: &mmtp.Receive{
						Filter: nil,
					},
				},
			},
		},
	}

	err := writeMessage(ctx, a.ws, receiveMsg)
	if err != nil {
		fmt.Println("Could not sendMsg response:", err)
		return mmtp.ResponseEnum_ERROR, nil, err
	}

	response, err := readMessage(ctx, a.ws)
	if err != nil {
		fmt.Println(err)
		return mmtp.ResponseEnum_ERROR, nil, err
	}

	msgs := []string{}
	for _, msg := range response.GetResponseMessage().GetApplicationMessages() {
		msgs = append(msgs, string(msg.GetBody()))
	}

	return mmtp.ResponseEnum_GOOD, msgs, nil
}

func NewAgent(mrn string) *Agent {
	return &Agent{Mrn: mrn, state: AgentState_NOTCONNECTED}
}

func (a *Agent) convertState(newState AgentState) {
	a.state = newState
}

func readMessage(ctx context.Context, ws *websocket.Conn) (*mmtp.MmtpMessage, error) {
	_, b, err := ws.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not read message from edge router: %w", err)
	}
	mmtpMessage := &mmtp.MmtpMessage{}
	if err = proto.Unmarshal(b, mmtpMessage); err != nil {
		return nil, fmt.Errorf("could not unmarshal message: %w", err)
	}
	return mmtpMessage, nil
}

func writeMessage(ctx context.Context, ws *websocket.Conn, mmtpMessage *mmtp.MmtpMessage) error {
	b, err := proto.Marshal(mmtpMessage)
	if err != nil {
		return fmt.Errorf("could not marshal message: %w", err)
	}
	err = ws.Write(ctx, websocket.MessageBinary, b)
	if err != nil {
		return fmt.Errorf("could not write message: %w", err)
	}
	return nil
}

func connectWS(ctx context.Context, url string) (*websocket.Conn, error) {
	edgeRouterWs, _, err := websocket.Dial(ctx, "ws://"+url, nil)
	if err != nil {
		return nil, errors.New("could not connect to MMS Edge Router")
	}
	return edgeRouterWs, nil
}

func (a *Agent) connectOverMMTP(ctx context.Context) (*mmtp.MmtpMessage, error) {
	connectMsg := &mmtp.MmtpMessage{
		MsgType: mmtp.MsgType_PROTOCOL_MESSAGE,
		Uuid:    uuid.NewString(),
		Body: &mmtp.MmtpMessage_ProtocolMessage{
			ProtocolMessage: &mmtp.ProtocolMessage{
				ProtocolMsgType: mmtp.ProtocolMessageType_CONNECT_MESSAGE,
				Body: &mmtp.ProtocolMessage_ConnectMessage{
					ConnectMessage: &mmtp.Connect{
						OwnMrn: &a.Mrn,
					},
				},
			},
		},
	}

	err := writeMessage(ctx, a.ws, connectMsg)
	if err != nil {
		fmt.Println("Could not connect:", err)
		return nil, err
	}

	response, err := readMessage(ctx, a.ws)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	return response, err
}