package events

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	eventMessages "github.com/FactomProject/factomd/common/messages/eventmessages"
	eventsInput "github.com/FactomProject/factomd/common/messages/eventmessages/input"
	"github.com/FactomProject/factomd/p2p"
	"github.com/gogo/protobuf/proto"
	"net"
	"time"
)

var connectionError = errors.New("")

const (
	defaultConnectionProtocol = "tcp"
	defaultConnectionHost     = "127.0.0.1"
	defaultConnectionPort     = "8040"
	sendRetries               = 3
	dialRetryPostponeDuration = time.Minute
	redialSleepDuration       = 5 * time.Second
)

type EventService interface {
	Send(event *eventsInput.EventInput)
}

type EventProxy struct {
	eventsOutQueue     chan eventsInput.EventInput
	postponeRetryUntil time.Time
	connection         net.Conn
	protocol           string
	address            string
}

func NewEventProxy() EventService {
	eventProxy := &EventProxy{
		eventsOutQueue: make(chan eventsInput.EventInput, p2p.StandardChannelSize),
		protocol:       defaultConnectionProtocol,
		address:        fmt.Sprintf("%s:%s", defaultConnectionHost, defaultConnectionPort),
	}
	go eventProxy.processEventsChannel()
	return eventProxy
}

func (ep *EventProxy) Send(event *eventsInput.EventInput) {
	select {
	case ep.eventsOutQueue <- *event:
	default:
	}
}

func (ep *EventProxy) processEventsChannel() {
	for event := range ep.eventsOutQueue {
		ep.processEvent(event)
	}
}

func (ep *EventProxy) processEvent(event eventsInput.EventInput) {
	factomEvent, err := MapToFactomEvent(event)
	if err != nil || factomEvent == nil {
		// TODO handle error
		fmt.Printf("TODO error logging: %v\n", err)
		return
	}

	if err := ep.sendEvent(factomEvent); err != nil {
		// TODO handle error
		fmt.Printf("TODO error logging: %v\n", err)
		return
	}
}

func (ep *EventProxy) sendEvent(event *eventMessages.FactomEvent) error {
	data, err := ep.marshallEvent(event)
	if err != nil {
		return fmt.Errorf("TODO error logging: %v", err)
	}

	// retry sending event ... times
	sendSuccessful := false
	for retry := 0; retry < sendRetries || sendSuccessful; retry++ {
		if err = ep.connect(); err != nil {
			// TODO handle error
			return fmt.Errorf("TODO error logging: %v", err)
		}

		// send the factom event to the live api
		if err = ep.writeEvent(data); err == nil {
			sendSuccessful = true
		} else {
			// TODO handle / log error
			fmt.Printf("TODO error logging: %v\n", err)
			if err == connectionError {
				// reset connection and retry
				ep.connection = nil
			}
		}
	}

	return nil
}

func (ep *EventProxy) connect() error {
	if ep.connection == nil {
		conn, err := net.Dial(ep.protocol, ep.address)
		if err != nil {
			return fmt.Errorf("failed to connect to %s at %s: %v", ep.protocol, ep.address, err)
		}
		ep.connection = conn
		ep.postponeRetryUntil = time.Unix(0, 0)
	}
	return nil
}

func (ep *EventProxy) marshallEvent(event *eventMessages.FactomEvent) (data []byte, err error) {
	data, err = proto.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("failed to marshell event: %v", err)
	}
	return data, err
}

func (ep *EventProxy) writeEvent(data []byte) (err error) {
	writer := bufio.NewWriter(ep.connection)

	dataSize := int32(len(data))
	err = binary.Write(writer, binary.LittleEndian, dataSize)
	if err != nil {
		return fmt.Errorf("failed to write data size: %v", err)
	}

	_, err = writer.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write data: %v", err)
	}
	err = writer.Flush()
	return nil
}
