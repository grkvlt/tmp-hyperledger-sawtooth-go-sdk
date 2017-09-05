/**
 * Copyright 2017 Intel Corporation
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
 * ------------------------------------------------------------------------------
 */

package processor

import (
	"fmt"
	"github.com/golang/protobuf/proto"
	"sawtooth_sdk/messaging"
	"sawtooth_sdk/protobuf/state_context_pb2"
	"sawtooth_sdk/protobuf/validator_pb2"
)

// Context provides an abstract interface for getting and setting validator
// state. All validator interactions by a handler should be through a Context
// instance. Currently, the Context class is NOT thread-safe and Context classes
// may not share the same messaging.Connection object.
type Context struct {
	connection messaging.Connection
	contextId  string
}

// Construct a new context object given an initialized Stream and Context ID.
func NewContext(connection messaging.Connection, contextId string) *Context {
	return &Context{
		connection: connection,
		contextId:  contextId,
	}
}

// Get queries the validator state for data at each of the addresses in the
// given slice. A string->[]byte map is returned. If an address is not set,
// it will not exist in the map.
//
//     results, err := context.Get(addresses)
//     if err != nil {
//         fmt.Println("Error getting data!")
//     }
//     data, ok := results[address]
//     if !ok {
//         fmt.Prinln("No data stored at address!")
//     }
//
func (self *Context) Get(addresses []string) (map[string][]byte, error) {
	// Construct the message
	request := &state_context_pb2.TpStateGetRequest{
		ContextId: self.contextId,
		Addresses: addresses,
	}
	bytes, err := proto.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("Failed to marshal TpStateGetRequest: %v", err)
	}

	// Send the message and get the response
	corrId, err := self.connection.SendNewMsg(
		validator_pb2.Message_TP_STATE_GET_REQUEST, bytes,
	)
	if err != nil {
		return nil, fmt.Errorf("Failed to send TpStateGetRequest: %v", err)
	}

	_, msg, err := self.connection.RecvMsgWithId(corrId)
	if err != nil {
		return nil, fmt.Errorf("Failed to received TpStateGetResponse: %v", err)
	}

	if msg.GetCorrelationId() != corrId {
		return nil, fmt.Errorf(
			"Expected message with correlation id %v but got %v",
			corrId, msg.GetCorrelationId(),
		)
	}

	if msg.GetMessageType() != validator_pb2.Message_TP_STATE_GET_RESPONSE {
		return nil, fmt.Errorf(
			"Expected TpStateGetResponse but got %v", msg.GetMessageType(),
		)
	}

	// Parse the result
	response := &state_context_pb2.TpStateGetResponse{}
	err = proto.Unmarshal(msg.GetContent(), response)
	if err != nil {
		return nil, fmt.Errorf("Failed to unmarshal TpStateGetResponse: %v", err)
	}

	// Use a switch in case new Status values are added
	switch response.Status {
	case state_context_pb2.TpStateGetResponse_AUTHORIZATION_ERROR:
		return nil, fmt.Errorf(
			"Tried to get unauthorized address: %v", addresses,
		)
	}

	// Construct and return a map
	results := make(map[string][]byte)
	for _, entry := range response.GetEntries() {
		if len(entry.GetData()) != 0 {
			results[entry.GetAddress()] = entry.GetData()
		}
	}

	return results, nil
}

// Set requests that each address in the validator state be set to the given
// value. A slice of addresses set is returned or an error if there was a
// problem setting the addresses. For example:
//
//     responses, err := context.Set(dataMap)
//     if err != nil {
//         fmt.Println("Error setting addresses!")
//     }
//     set, ok := results[address]
//     if !ok {
//         fmt.Prinln("Address was not set!")
//     }
//
func (self *Context) Set(pairs map[string][]byte) ([]string, error) {
	// Construct the message
	entries := make([]*state_context_pb2.Entry, 0, len(pairs))
	for address, data := range pairs {
		entries = append(entries, &state_context_pb2.Entry{
			Address: address,
			Data:    data,
		})
	}

	request := &state_context_pb2.TpStateSetRequest{
		ContextId: self.contextId,
		Entries:   entries,
	}
	bytes, err := proto.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("Failed to marshal: %v", err)
	}

	// Send the message and get the response
	corrId, err := self.connection.SendNewMsg(
		validator_pb2.Message_TP_STATE_SET_REQUEST, bytes,
	)
	if err != nil {
		return nil, fmt.Errorf("Failed to send set: %v", err)
	}

	_, msg, err := self.connection.RecvMsgWithId(corrId)
	if err != nil {
		return nil, fmt.Errorf("Failed to receive TpStateSetResponse: %v", err)
	}
	if msg.GetCorrelationId() != corrId {
		return nil, fmt.Errorf(
			"Expected message with correlation id %v but got %v",
			corrId, msg.GetCorrelationId(),
		)
	}

	if msg.GetMessageType() != validator_pb2.Message_TP_STATE_SET_RESPONSE {
		return nil, fmt.Errorf(
			"Expected TP_STATE_SET_RESPONSE but got %v", msg.GetMessageType(),
		)
	}

	// Parse the result
	response := &state_context_pb2.TpStateSetResponse{}
	err = proto.Unmarshal(msg.Content, response)
	if err != nil {
		return nil, fmt.Errorf("Failed to unmarshal TpStateSetResponse: %v", err)
	}

	// Use a switch in case new Status values are added
	switch response.GetStatus() {
	case state_context_pb2.TpStateSetResponse_AUTHORIZATION_ERROR:
		addresses := make([]string, 0, len(pairs))
		for a, _ := range pairs {
			addresses = append(addresses, a)
		}
		return nil, fmt.Errorf("Tried to set unauthorized address: %v", addresses)
	}

	return response.GetAddresses(), nil
}
