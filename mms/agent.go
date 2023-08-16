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
	"errors"
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

var errNotConnected = errors.New("agent could not connect to an Edge Router")
var errNotAuthenticated = errors.New("agent is not authenticated")
var errNotDisconnected = errors.New("could not disconnect to MMS Edge Router")
var errNotDisconnectedWs = errors.New("could not disconnect with websocket")

// Status returns the the current status of the MMS Agent
func (a *Agent) Status() []string {
	var edgeRouterMRNs []string
	fmt.Println(edgeRouterMRNs)
	return edgeRouterMRNs
}

// Discover looks up possible MMS Edge Routers
func (a *Agent) Discover() {

}

// Connect make connect anonymously to an MMS Edge Router
func (a *Agent) Connect(ctx context.Context, url string) (mmtp.ResponseEnum, error) {
	err := errors.New("err")
	ws, err := connectWS(ctx, url)
	a.ws = ws

	if err != nil {
		fmt.Errorf("could not create web socket connection: %w", err)
		return mmtp.ResponseEnum_ERROR, err
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

// TODO: implement this function
// Authenticate imports an MCP cert and does authentication.
// MRN attribute in the cert will be stored to the MRN of Agent
func (a *Agent) Authenticate(ctx context.Context, certificate *x509.Certificate) (mmtp.ResponseEnum, error) {
	a.convertState(AgentState_AUTHENTICATED)
	return mmtp.ResponseEnum_GOOD, nil
}

// ReconnectAnonymous reconnects anonymously to an MMS Edge Router
func (a *Agent) ReconnectAnonymous() {

}

// Disconnect disconnects from the MMS Edge Router
func (a *Agent) Disconnect(ctx context.Context) (mmtp.ResponseEnum, error) {
	if a.state == AgentState_NOTCONNECTED {
		fmt.Errorf("already disconnected")
		return mmtp.ResponseEnum_GOOD, nil
	}

	res, errMmtp := a.disconnectOverMMTP(ctx)
	if res == nil || errMmtp != nil {
		fmt.Println("there was a problem in connection to MMS Edge Router", errMmtp)
		return mmtp.ResponseEnum_ERROR, errMmtp
	} else if res.GetResponseMessage().Response != mmtp.ResponseEnum_GOOD {
		fmt.Println("MMS Edge Router did not accept Disconnect:", res.GetResponseMessage().GetReasonText())
		return mmtp.ResponseEnum_ERROR, errMmtp
	}

	err := disconnectWS(ctx, a.ws)
	if err != nil {
		fmt.Errorf("could not disconnect web socket connection: %w", err)
		return mmtp.ResponseEnum_ERROR, err
	}
	return mmtp.ResponseEnum_GOOD, nil
}

// Send transfers a message to another Agent with receivingMrn
func (a *Agent) Send(ctx context.Context, timeToLive time.Duration, receivingMrn string, bytes []byte) (mmtp.ResponseEnum, error) {
	switch a.state {
	case AgentState_NOTCONNECTED:
		return mmtp.ResponseEnum_ERROR, errNotConnected
	case AgentState_CONNECTED:
		return mmtp.ResponseEnum_ERROR, errNotAuthenticated
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
		fmt.Println("Could not sendMsg response:", err)
		return mmtp.ResponseEnum_ERROR, err
	}
	return mmtp.ResponseEnum_GOOD, nil
}

// Receive fetches a list of messages sent to its own MRN
func (a *Agent) Receive(ctx context.Context) (mmtp.ResponseEnum, []string, error) {
	switch a.state {
	case AgentState_NOTCONNECTED:
		return mmtp.ResponseEnum_ERROR, nil, errNotConnected
	case AgentState_CONNECTED:
		return mmtp.ResponseEnum_ERROR, nil, errNotAuthenticated
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

func connectWS(ctx context.Context, url string) (*websocket.Conn, error) {
	edgeRouterWs, _, err := websocket.Dial(ctx, "ws://"+url, nil)
	if err != nil {
		return nil, errors.New("could not connect to MMS Edge Router")
	}
	edgeRouterWs.SetReadLimit(1000000)
	return edgeRouterWs, nil
}

func disconnectWS(ctx context.Context, ws *websocket.Conn) error {
	err := ws.Close(websocket.StatusNormalClosure, "normal closure")
	if err != nil {
		return errNotDisconnectedWs
	}
	return nil
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

func (a *Agent) disconnectOverMMTP(ctx context.Context) (*mmtp.MmtpMessage, error) {
	connectMsg := &mmtp.MmtpMessage{
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

	err := writeMessage(ctx, a.ws, connectMsg)
	if err != nil {
		fmt.Println("Could not disconnect:", err)
		return nil, err
	}

	response, err := readMessage(ctx, a.ws)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	return response, err
}
