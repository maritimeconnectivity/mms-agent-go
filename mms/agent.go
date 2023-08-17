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
	"crypto/x509"
	"fmt"
	"github.com/google/uuid"
	"github.com/maritimeconnectivity/mms-agent-go/generated/mmtp"
	"google.golang.org/protobuf/proto"
	"nhooyr.io/websocket"
	"sync"
	"time"
)

// Agent type representing an agent for a connection to an Edge Router
type Agent struct {
	Mrn            string                       // the MRN of the Agent
	Interests      []string                     // the Interests that the Agent wants to subscribe to
	Messages       map[string]*mmtp.MmtpMessage // the incoming messages for this Agent
	msgMu          *sync.RWMutex                // RWMutex for locking the Messages map
	reconnectToken string                       // token for reconnecting to a previous session
	state          AgentState                   // state of Agent
	ws             *websocket.Conn              // websocket connection bind
}

// AgentState type representing a state of Agent
type AgentState int16

const (
	AgentState_NOTCONNECTED  AgentState = 0
	AgentState_CONNECTED     AgentState = 1
	AgentState_AUTHENTICATED AgentState = 2
)

// Status returns current status of the MMS Agent
func (a *Agent) Status() AgentState {
	return a.state
}

// ConnectAuthenticated make connect to an MMS Edge Router
func (a *Agent) ConnectAuthenticated(ctx context.Context, url string) (mmtp.ResponseEnum, error) {
	ws, err := connectWS(ctx, url)
	a.ws = ws

	if err != nil {
		return mmtp.ResponseEnum_ERROR, fmt.Errorf("failed to connect to MMS Edge Router: %w", err)
	}

	res, errMmtp := a.connectOverMMTP(ctx)
	if res == nil || errMmtp != nil {
		return mmtp.ResponseEnum_ERROR, fmt.Errorf("there was a problem in connection to MMS Edge Router: %w", errMmtp)
	} else if res.GetResponseMessage().Response != mmtp.ResponseEnum_GOOD {
		return mmtp.ResponseEnum_ERROR, fmt.Errorf("MMS Edge Router did not accept ConnectAuthenticated: ", res.GetResponseMessage().GetReasonText())
	}

	a.convertState(AgentState_CONNECTED)
	a.Authenticate(ctx, nil)
	return res.GetResponseMessage().Response, err
}

// ConnectAnonymous make connect anonymously to an MMS Edge Router
func (a *Agent) ConnectAnonymous(ctx context.Context, url string) (mmtp.ResponseEnum, error) {
	ws, err := connectWS(ctx, url)
	a.ws = ws

	if err != nil {
		return mmtp.ResponseEnum_ERROR, fmt.Errorf("could not create web socket connection: %w", err)
	}

	res, errMmtp := a.connectAnonymousOverMMTP(ctx)
	if res == nil || errMmtp != nil {
		return mmtp.ResponseEnum_ERROR, fmt.Errorf("failed to connect to MMS Edge Router: %w", errMmtp)
	} else if res.GetResponseMessage().Response != mmtp.ResponseEnum_GOOD {
		return mmtp.ResponseEnum_ERROR, fmt.Errorf("MMS Edge Router did not accept ConnectAuthenticated: ", res.GetResponseMessage().GetReasonText())
	}

	a.convertState(AgentState_CONNECTED)
	return res.GetResponseMessage().Response, err
}

// TODO: implement this function
// Authenticate imports an MCP cert and does authentication.
// MRN attribute in the cert will be stored to the MRN of Agent
func (a *Agent) Authenticate(ctx context.Context, certificate *x509.Certificate) (mmtp.ResponseEnum, error) {
	a.convertState(AgentState_AUTHENTICATED)
	return mmtp.ResponseEnum_GOOD, nil
}

// Disconnect disconnects from the MMS Edge Router
func (a *Agent) Disconnect(ctx context.Context) (mmtp.ResponseEnum, error) {
	if a.state == AgentState_NOTCONNECTED {
		return mmtp.ResponseEnum_GOOD, nil
	}

	res, errMmtp := a.disconnectOverMMTP(ctx)
	if res == nil || errMmtp != nil {
		return mmtp.ResponseEnum_ERROR, fmt.Errorf("there was a problem in connection to MMS Edge Router: %w", errMmtp)
	} else if res.GetResponseMessage().Response != mmtp.ResponseEnum_GOOD {
		return mmtp.ResponseEnum_ERROR, fmt.Errorf("MMS Edge Router did not accept Disconnect: ", res.GetResponseMessage().GetReasonText())
	}

	err := disconnectWS(ctx, a.ws)
	if err != nil {
		return mmtp.ResponseEnum_ERROR, fmt.Errorf("could not disconnect web socket connection: %w", err)
	}
	return mmtp.ResponseEnum_GOOD, nil
}

// SendDirect transfers a message to another Agent with receivingMrn
func (a *Agent) SendDirect(ctx context.Context, timeToLive time.Duration, receivingMrn string, bytes []byte) (mmtp.ResponseEnum, error) {
	switch a.state {
	case AgentState_NOTCONNECTED:
		return mmtp.ResponseEnum_ERROR, fmt.Errorf("agent is not connected to an Edge Router")
	case AgentState_CONNECTED:
		return mmtp.ResponseEnum_ERROR, fmt.Errorf("agent is not authenticated")
	}

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
		return mmtp.ResponseEnum_ERROR, fmt.Errorf("failed to send message: %w", err)
	}
	return mmtp.ResponseEnum_GOOD, nil
}

// SendSubject transfers a message with regard to a subject
func (a *Agent) SendSubject(ctx context.Context, timeToLive time.Duration, subject string, bytes []byte) (mmtp.ResponseEnum, error) {
	switch a.state {
	case AgentState_NOTCONNECTED:
		return mmtp.ResponseEnum_ERROR, fmt.Errorf("agent is not connected to an Edge Router")
	case AgentState_CONNECTED:
		return mmtp.ResponseEnum_ERROR, fmt.Errorf("agent is not authenticated")
	}

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
								SubjectOrRecipient: &mmtp.ApplicationMessageHeader_Subject{
									Subject: subject,
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
		return mmtp.ResponseEnum_ERROR, fmt.Errorf("failed to send message: %w", err)
	}
	return mmtp.ResponseEnum_GOOD, nil
}

// Receive fetches a list of messages sent to its own MRN
func (a *Agent) Receive(ctx context.Context, filter *mmtp.Filter) (mmtp.ResponseEnum, [][]byte, error) {
	switch a.state {
	case AgentState_NOTCONNECTED:
		return mmtp.ResponseEnum_ERROR, nil, fmt.Errorf("agent is not connected to an Edge Router")
	}

	receiveMsg := &mmtp.MmtpMessage{
		MsgType: mmtp.MsgType_PROTOCOL_MESSAGE,
		Uuid:    uuid.NewString(),
		Body: &mmtp.MmtpMessage_ProtocolMessage{
			ProtocolMessage: &mmtp.ProtocolMessage{
				ProtocolMsgType: mmtp.ProtocolMessageType_RECEIVE_MESSAGE,
				Body: &mmtp.ProtocolMessage_ReceiveMessage{
					ReceiveMessage: &mmtp.Receive{
						Filter: filter,
					},
				},
			},
		},
	}

	response, err := writeAndReadMessage(ctx, a.ws, receiveMsg)
	if err != nil {
		return mmtp.ResponseEnum_ERROR, nil, fmt.Errorf("could not receive messages: %w", err)
	}

	msgs := make([][]byte, 0, len(response.GetResponseMessage().GetApplicationMessages()))
	for _, msg := range response.GetResponseMessage().GetApplicationMessages() {
		msgs = append(msgs, msg.GetBody())
	}

	return mmtp.ResponseEnum_GOOD, msgs, nil
}

func (a *Agent) Subscribe(ctx context.Context, subject string) (mmtp.ResponseEnum, error) {
	switch a.state {
	case AgentState_NOTCONNECTED:
		return mmtp.ResponseEnum_ERROR, fmt.Errorf("agent is not connected to an Edge Router")
	}

	subMsg := &mmtp.MmtpMessage{
		MsgType: mmtp.MsgType_PROTOCOL_MESSAGE,
		Uuid:    uuid.NewString(),
		Body: &mmtp.MmtpMessage_ProtocolMessage{
			ProtocolMessage: &mmtp.ProtocolMessage{
				ProtocolMsgType: mmtp.ProtocolMessageType_SUBSCRIBE_MESSAGE,
				Body: &mmtp.ProtocolMessage_SubscribeMessage{
					SubscribeMessage: &mmtp.Subscribe{
						Subject: subject,
					},
				},
			},
		},
	}

	err := writeMessage(ctx, a.ws, subMsg)
	if err != nil {
		return mmtp.ResponseEnum_ERROR, fmt.Errorf("could not subscribe: %w", err)
	}
	a.Interests = append(a.Interests, subject)
	return mmtp.ResponseEnum_GOOD, nil
}

func (a *Agent) Unsubscribe(ctx context.Context, subject string) (mmtp.ResponseEnum, error) {
	switch a.state {
	case AgentState_NOTCONNECTED:
		return mmtp.ResponseEnum_ERROR, fmt.Errorf("agent is not connected to an Edge Router")
	}

	subMsg := &mmtp.MmtpMessage{
		MsgType: mmtp.MsgType_PROTOCOL_MESSAGE,
		Uuid:    uuid.NewString(),
		Body: &mmtp.MmtpMessage_ProtocolMessage{
			ProtocolMessage: &mmtp.ProtocolMessage{
				ProtocolMsgType: mmtp.ProtocolMessageType_UNSUBSCRIBE_MESSAGE,
				Body: &mmtp.ProtocolMessage_UnsubscribeMessage{
					UnsubscribeMessage: &mmtp.Unsubscribe{
						Subject: subject,
					},
				},
			},
		},
	}

	err := writeMessage(ctx, a.ws, subMsg)
	if err != nil {
		return mmtp.ResponseEnum_ERROR, fmt.Errorf("could not unsubcribe: %w", err)
	}
	a.Interests = remove(a.Interests, subject)
	return mmtp.ResponseEnum_GOOD, nil
}

func (a *Agent) SubscribeMessages(ctx context.Context) {

}

// ReconnectAnonymous reconnects anonymously to an MMS Edge Router
func (a *Agent) ReconnectAnonymous() {

}
func (a *Agent) UnsubscribeMessages(ctx context.Context) {

}

// Discover looks up possible MMS Edge Routers
func (a *Agent) Discover() {

}

// NewAgent initiates a new Agent object
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

func writeAndReadMessage(ctx context.Context, ws *websocket.Conn, mmtpMessage *mmtp.MmtpMessage) (*mmtp.MmtpMessage, error) {
	err := writeMessage(ctx, ws, mmtpMessage)
	if err != nil {
		return nil, fmt.Errorf("failed to write message: %w", err)
	}

	response, err := readMessage(ctx, ws)
	if err != nil {
		return nil, fmt.Errorf("failed to read message: %w", err)
	}

	return response, err
}

func connectWS(ctx context.Context, url string) (*websocket.Conn, error) {
	edgeRouterWs, _, err := websocket.Dial(ctx, "ws://"+url, nil)
	if err != nil {
		return nil, fmt.Errorf("could not connect to MMS Edge Router: %w", err)
	}
	edgeRouterWs.SetReadLimit(1000000)
	return edgeRouterWs, nil
}

func disconnectWS(ctx context.Context, ws *websocket.Conn) error {
	err := ws.Close(websocket.StatusNormalClosure, "normal closure")
	if err != nil {
		return fmt.Errorf("could not disconnect with websocket: %w", err)
	}
	return nil
}

/*
	func TestRemove() {
		test := make([]string, 0)
		test = append(test, "Test1")
		fmt.Println(remove(test, "Test2"))
		fmt.Println(remove(test, "Test1"))
	}
*/
func remove(slice []string, s string) []string {
	var idx = -1
	for i := range slice {
		if slice[i] == s {
			idx = i
			break
		}
	}
	if idx != -1 {
		return append(slice[:idx], slice[idx+1:]...)
	}
	return slice
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

	return writeAndReadMessage(ctx, a.ws, connectMsg)
}

func (a *Agent) connectAnonymousOverMMTP(ctx context.Context) (*mmtp.MmtpMessage, error) {
	connectMsg := &mmtp.MmtpMessage{
		MsgType: mmtp.MsgType_PROTOCOL_MESSAGE,
		Uuid:    uuid.NewString(),
		Body: &mmtp.MmtpMessage_ProtocolMessage{
			ProtocolMessage: &mmtp.ProtocolMessage{
				ProtocolMsgType: mmtp.ProtocolMessageType_CONNECT_MESSAGE,
				Body: &mmtp.ProtocolMessage_ConnectMessage{
					ConnectMessage: &mmtp.Connect{},
				},
			},
		},
	}

	return writeAndReadMessage(ctx, a.ws, connectMsg)
}

func (a *Agent) disconnectOverMMTP(ctx context.Context) (*mmtp.MmtpMessage, error) {
	disconnectMsg := &mmtp.MmtpMessage{
		MsgType: mmtp.MsgType_PROTOCOL_MESSAGE,
		Uuid:    uuid.NewString(),
		Body: &mmtp.MmtpMessage_ProtocolMessage{
			ProtocolMessage: &mmtp.ProtocolMessage{
				ProtocolMsgType: mmtp.ProtocolMessageType_DISCONNECT_MESSAGE,
				Body: &mmtp.ProtocolMessage_DisconnectMessage{
					DisconnectMessage: &mmtp.Disconnect{},
				},
			},
		},
	}

	return writeAndReadMessage(ctx, a.ws, disconnectMsg)
}
