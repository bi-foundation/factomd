package eventservices

import (
	"encoding/binary"
	"errors"
	"github.com/FactomProject/factomd/common/interfaces"
	"github.com/FactomProject/factomd/common/messages"
	"github.com/FactomProject/factomd/common/primitives"
	"github.com/FactomProject/factomd/events"
	"github.com/FactomProject/factomd/events/eventmessages/generated/eventmessages"
	"github.com/gogo/protobuf/types"
	"time"
)

func MapToFactomEvent(eventInput events.EventInput, broadcastContent BroadcastContent, sendStateChangeEvents bool, ownerState ServiceOwnerState) (*eventmessages.FactomEvent, error) {
	var factomEvent *eventmessages.FactomEvent
	var err error
	switch eventInput.(type) {
	case *events.RegistrationEvent:
		registrationEvent := eventInput.(*events.RegistrationEvent)
		factomEvent, err = mapRegistrationEvent(registrationEvent, broadcastContent)
	case *events.StateChangeMsgEvent:
		stateChangeEvent := eventInput.(*events.StateChangeMsgEvent)
		factomEvent, err = mapStateChangeEvent(stateChangeEvent, broadcastContent, sendStateChangeEvents, ownerState)
	case *events.StateChangeEvent:
		stateChangeEvent := eventInput.(*events.StateChangeEvent)
		factomEvent, err = mapDBStateEvent(stateChangeEvent, broadcastContent)
	case *events.ProcessListEvent:
		processMessageEvent := eventInput.(*events.ProcessListEvent)
		factomEvent, err = mapProcessMessageEvent(processMessageEvent)
	case *events.NodeMessageEvent:
		nodeMessageEvent := eventInput.(*events.NodeMessageEvent)
		return mapNodeMessageEvent(nodeMessageEvent)
	default:
		return nil, errors.New("no payload found in source event")
	}
	factomEvent.FactomNodeName = ownerState.GetFactomNodeName()
	factomEvent.IdentityChainID = ownerState.GetIdentityChainID().Bytes()
	return factomEvent, err
}

func mapRegistrationEvent(registrationEvent *events.RegistrationEvent, broadcastContent BroadcastContent) (*eventmessages.FactomEvent, error) {
	event := &eventmessages.FactomEvent{}
	event.EventSource = registrationEvent.GetStreamSource()
	msg := registrationEvent.GetPayload()
	if msg != nil {
		shouldIncludeContent := broadcastContent > BroadcastNever

		switch msg.(type) {
		case *messages.CommitChainMsg:
			commitChainMsg := msg.(*messages.CommitChainMsg)
			event.Event = mapCommitChain(eventmessages.EntityState_REQUESTED, commitChainMsg)
		case *messages.CommitEntryMsg:
			commitEntryMsg := msg.(*messages.CommitEntryMsg)
			event.Event = mapCommitEntryEvent(eventmessages.EntityState_REQUESTED, commitEntryMsg)
		case *messages.RevealEntryMsg:
			revealEntryMsg := msg.(*messages.RevealEntryMsg)
			if shouldIncludeContent {
				event.Event = mapRevealEntryEvent(eventmessages.EntityState_REQUESTED, revealEntryMsg)
			} else {
				return nil, nil
			}
		default:
			return nil, errors.New("unknown message type")
		}
	}
	return event, nil
}

func mapStateChangeEvent(stateChangeEvent *events.StateChangeMsgEvent, broadcastContent BroadcastContent, sendStateChangeEvents bool,
	ownerState ServiceOwnerState) (*eventmessages.FactomEvent, error) {
	event := &eventmessages.FactomEvent{}
	event.EventSource = stateChangeEvent.GetStreamSource()
	msg := stateChangeEvent.GetPayload()
	if msg != nil {
		shouldIncludeContent := broadcastContent > BroadcastOnce

		switch msg.(type) {
		case *messages.CommitChainMsg:
			commitChainMsg := msg.(*messages.CommitChainMsg)
			if sendStateChangeEvents {
				event.Event = mapCommitChainState(stateChangeEvent.GetEntityState(), commitChainMsg)
			} else {
				event.Event = mapCommitChain(stateChangeEvent.GetEntityState(), commitChainMsg)
			}
		case *messages.CommitEntryMsg:
			commitEntryMsg := msg.(*messages.CommitEntryMsg)
			if sendStateChangeEvents {
				event.Event = mapCommitEntryEventState(stateChangeEvent.GetEntityState(), commitEntryMsg)
			} else {
				event.Event = mapCommitEntryEvent(stateChangeEvent.GetEntityState(), commitEntryMsg)
			}
		case *messages.RevealEntryMsg:
			revealEntryMsg := msg.(*messages.RevealEntryMsg)
			if sendStateChangeEvents {
				event.Event = mapRevealEntryEventState(stateChangeEvent.GetEntityState(), revealEntryMsg)
			} else if shouldIncludeContent {
				event.Event = mapRevealEntryEvent(stateChangeEvent.GetEntityState(), revealEntryMsg)
			}
		case *messages.DBStateMsg:
			dbStateMessage := msg.(*messages.DBStateMsg)
			event.Event = mapDBStateFromMsg(dbStateMessage, shouldIncludeContent, ownerState)
		default:
			return nil, errors.New("unknown message type")
		}
	}
	return event, nil
}

func mapDBStateEvent(stateChangeEvent *events.StateChangeEvent, broadcastContent BroadcastContent) (*eventmessages.FactomEvent, error) {
	event := &eventmessages.FactomEvent{}
	event.EventSource = stateChangeEvent.GetStreamSource()
	state := stateChangeEvent.GetPayload()
	stateChangeEvent.GetEntityState()
	if state != nil {
		shouldIncludeContent := broadcastContent > BroadcastOnce
		event.Event = mapDBState(state, shouldIncludeContent)
	}
	return event, nil
}

func mapProcessMessageEvent(processMessageEvent *events.ProcessListEvent) (*eventmessages.FactomEvent, error) {
	event := &eventmessages.FactomEvent{
		EventSource: processMessageEvent.GetStreamSource(),
		Event: &eventmessages.FactomEvent_ProcessListEvent{
			ProcessListEvent: processMessageEvent.GetProcessListEvent(),
		},
	}
	return event, nil
}

func mapNodeMessageEvent(nodeMessageEvent *events.NodeMessageEvent) (*eventmessages.FactomEvent, error) {
	event := &eventmessages.FactomEvent{
		EventSource: nodeMessageEvent.GetStreamSource(),
		Event: &eventmessages.FactomEvent_NodeMessage{
			NodeMessage: nodeMessageEvent.GetNodeMessage(),
		},
	}
	return event, nil
}

func mapDBStateFromMsg(dbStateMessage *messages.DBStateMsg, shouldIncludeContent bool, ownerState ServiceOwnerState) *eventmessages.FactomEvent_DirectoryBlockCommit {
	event := &eventmessages.FactomEvent_DirectoryBlockCommit{DirectoryBlockCommit: &eventmessages.DirectoryBlockCommit{
		DirectoryBlock:    mapDirectoryBlock(dbStateMessage.DirectoryBlock, ownerState),
		AdminBlock:        MapAdminBlock(dbStateMessage.AdminBlock),
		FactoidBlock:      mapFactoidBlock(dbStateMessage.FactoidBlock),
		EntryCreditBlock:  mapEntryCreditBlock(dbStateMessage.EntryCreditBlock),
		EntryBlocks:       mapEntryBlocks(dbStateMessage.EBlocks),
		EntryBlockEntries: mapEntryBlockEntries(nil, shouldIncludeContent, ownerState),
	}}
	return event
}

func mapDBState(dbState interfaces.IDBState, shouldIncludeContent bool, ownerState ServiceOwnerState) *eventmessages.FactomEvent_DirectoryBlockCommit {
	event := &eventmessages.FactomEvent_DirectoryBlockCommit{DirectoryBlockCommit: &eventmessages.DirectoryBlockCommit{
		DirectoryBlock:    mapDirectoryBlock(dbState.GetDirectoryBlock(), nil),
		AdminBlock:        MapAdminBlock(dbState.GetAdminBlock()),
		FactoidBlock:      mapFactoidBlock(dbState.GetFactoidBlock()),
		EntryCreditBlock:  mapEntryCreditBlock(dbState.GetEntryCreditBlock()),
		EntryBlocks:       mapEntryBlocks(dbState.GetEntryBlocks()),
		EntryBlockEntries: mapEntryBlockEntries(dbState.GetEntryBlocks(), shouldIncludeContent, ownerState),
	}}
	return event
}

func convertByteSlice6ToTimestamp(milliTime *primitives.ByteSlice6) *types.Timestamp {
	// TODO Is there an easier way to do this?
	slice8 := make([]byte, 8)
	copy(slice8[2:], milliTime[:])
	millis := int64(binary.BigEndian.Uint64(slice8))
	t := time.Unix(0, millis*1000000)
	return ConvertTimeToTimestamp(t)
}

func ConvertTimeToTimestamp(t time.Time) *types.Timestamp {
	return &types.Timestamp{Seconds: t.Unix(), Nanos: int32(t.Nanosecond())}
}
