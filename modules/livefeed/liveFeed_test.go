package livefeed

import (
	"fmt"
	"github.com/FactomProject/factomd/common/constants/runstate"
	"github.com/FactomProject/factomd/common/globals"
	"github.com/FactomProject/factomd/common/interfaces"
	"github.com/FactomProject/factomd/common/primitives"
	"github.com/FactomProject/factomd/modules/events"
	"github.com/FactomProject/factomd/modules/livefeed/eventconfig"
	"github.com/FactomProject/factomd/modules/livefeed/eventmessages/generated/eventmessages"
	"github.com/FactomProject/factomd/modules/livefeed/eventservices"
	"github.com/FactomProject/factomd/p2p"
	"github.com/FactomProject/factomd/pubsub"
	"github.com/FactomProject/factomd/pubsub/pubregistry"
	"github.com/FactomProject/factomd/testHelper"
	"github.com/FactomProject/factomd/util"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestEventEmitter_Send(t *testing.T) {
	testCases := map[string]struct {
		serviceState StateEventServices
		eventSender  eventservices.EventSender
		config       *util.FactomdConfig
		factomParams *globals.FactomParams
		Event        interface{}
		Assertion    func(*testing.T, *mockEventSender)
	}{
		"queue-filled": {
			serviceState: &StateMock{
				IdentityChainID: primitives.NewZeroHash(),
				PubRegistry:     pubregistry.New("UnitTestNode_LiveFeed"),
			},
			eventSender: &mockEventSender{
				eventsOutQueue:          make(chan *eventmessages.FactomEvent, 0),
				droppedFromQueueCounter: prometheus.NewCounter(prometheus.CounterOpts{}),
			},
			config:       buildTestConfig(),
			factomParams: nil,
			Event: &events.NodeMessage{
				MessageCode: events.NodeMessageCode_GENERAL,
				Level:       events.Level_ERROR,
				MessageText: fmt.Sprintf("test message"),
			},
			Assertion: func(t *testing.T, eventService *mockEventSender) {
				assert.Equal(t, 0, len(eventService.eventsOutQueue))
				assert.Equal(t, float64(1), getCounterValue(t, eventService.droppedFromQueueCounter))
			},
		},
		"not-running": {
			serviceState: &StateMock{
				RunState:    runstate.Stopping,
				PubRegistry: pubregistry.New("UnitTestNode_LiveFeed"),
			},
			eventSender: &mockEventSender{
				eventsOutQueue: make(chan *eventmessages.FactomEvent, p2p.StandardChannelSize),
			},
			config:       buildTestConfig(),
			factomParams: nil,
			Event:        nil,
			Assertion: func(t *testing.T, eventService *mockEventSender) {
				assert.Equal(t, 0, len(eventService.eventsOutQueue))
			},
		},
		"nil-event": {
			serviceState: &StateMock{
				PubRegistry: pubregistry.New("UnitTestNode_LiveFeed"),
			},
			eventSender: &mockEventSender{
				eventsOutQueue:      make(chan *eventmessages.FactomEvent, p2p.StandardChannelSize),
				replayDuringStartup: true,
			},
			config:       buildTestConfig(),
			factomParams: nil,
			Event:        nil,
			Assertion: func(t *testing.T, eventService *mockEventSender) {
				assert.Equal(t, 0, len(eventService.eventsOutQueue))
			},
		},
		"mute-replay-starting": {
			serviceState: &StateMock{
				RunLeader: false,
			},
			config:       buildTestConfig(),
			factomParams: nil,
			Event: events.CommitChain{
				RequestState: events.RequestState_REJECTED,
				DBHeight:     123,
				CommitChain:  testHelper.NewTestCommitChain(),
			},
			Assertion: func(t *testing.T, eventService *mockEventSender) {
				assert.Equal(t, 0, len(eventService.eventsOutQueue))
			},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			liveFeedService := NewTestLiveFeedService(testCase.eventSender)
			state := testCase.serviceState
			liveFeedService.Start(state, testCase.config, testCase.factomParams)
			publisher := selectPublisher(state, testCase.Event)
			publisher.Write(testCase.Event)
			testCase.Assertion(t, liveFeedService.eventSender.(*mockEventSender))
		})
	}
}

func selectPublisher(state StateEventServices, event interface{}) pubsub.IPublisher {
	registry := state.GetPubRegistry()
	switch event.(type) {
	case events.CommitChain:
		return registry.GetCommitChain()
	case events.CommitEntry:
		return registry.GetCommitEntry()
	}
	return registry.GetNodeMessage() // For tests not using a specific event, return node message channel which is the most generic
}

func buildTestConfig() *util.FactomdConfig {
	return &util.FactomdConfig{
		LiveFeedAPI: struct {
			EnableLiveFeedAPI        bool
			EventReceiverProtocol    string
			EventReceiverHost        string
			EventReceiverPort        int
			EventFormat              string
			EventReplayDuringStartup bool
			EventSendStateChange     bool
			EventBroadcastContent    string
		}{
			EnableLiveFeedAPI:        true,
			EventReceiverProtocol:    "tcp",
			EventReceiverHost:        "127.0.0.1",
			EventReceiverPort:        8040,
			EventFormat:              "protobuf",
			EventReplayDuringStartup: false,
			EventSendStateChange:     true,
			EventBroadcastContent:    "always",
		},
	}
}

func NewTestLiveFeedService(eventSender eventservices.EventSender) *liveFeedService {
	liveFeedService := new(liveFeedService)
	liveFeedService.factomEventPublisher = pubsub.PubFactory.Threaded(p2p.StandardChannelSize).Publish("/live-feed", pubsub.PubMultiWrap())
	liveFeedService.eventSender = eventSender
	pubsub.SubFactory.PrometheusCounter("factomd_livefeed_total_events_published", "Number of events published by the factomd backend")
	return liveFeedService
}

func TestEventsService_SendFillupQueue(t *testing.T) {
	n := 3

	eventSender := &mockEventSender{
		eventsOutQueue:          make(chan *eventmessages.FactomEvent, n),
		droppedFromQueueCounter: prometheus.NewCounter(prometheus.CounterOpts{}),
	}
	liveFeedService := NewTestLiveFeedService(eventSender)
	state := &StateMock{
		IdentityChainID: primitives.NewZeroHash(),
	}
	liveFeedService.Start(state, buildTestConfig(), nil)

	event := events.NodeMessage{
		MessageCode: events.NodeMessageCode_GENERAL,
		Level:       events.Level_ERROR,
		MessageText: fmt.Sprintf("test message"),
	}
	publisher := selectPublisher(state, event)
	for i := 0; i < n+1; i++ {
		publisher.Write(event)
	}

	assert.Equal(t, float64(1), getCounterValue(t, eventSender.droppedFromQueueCounter))
}

func getCounterValue(t *testing.T, counter prometheus.Counter) float64 {
	metric := &dto.Metric{}
	err := counter.Write(metric)
	if err != nil {
		assert.Fail(t, "fail to retrieve prometheus counter: %v", err)
	}
	return *metric.Counter.Value
}

type mockEventSender struct {
	eventsOutQueue          chan *eventmessages.FactomEvent
	droppedFromQueueCounter prometheus.Counter
	notSentCounter          prometheus.Counter
	replayDuringStartup     bool
}

func (m *mockEventSender) GetBroadcastContent() eventconfig.BroadcastContent {
	return eventconfig.BroadcastAlways
}
func (m *mockEventSender) IsSendStateChangeEvents() bool {
	return true
}
func (m *mockEventSender) ReplayDuringStartup() bool {
	return m.replayDuringStartup
}
func (m *mockEventSender) IncreaseDroppedFromQueueCounter() {
	m.droppedFromQueueCounter.Inc()
}
func (m *mockEventSender) GetEventQueue() chan *eventmessages.FactomEvent {
	return m.eventsOutQueue
}

func (m *mockEventSender) Shutdown() {}

type StateMock struct {
	StateEventServices
	IdentityChainID interfaces.IHash
	RunState        runstate.RunState
	RunLeader       bool
	Service         LiveFeedService
	PubRegistry     pubsub.IPubRegistry
}

func (s StateMock) GetRunState() runstate.RunState {
	return s.RunState
}

func (s StateMock) GetIdentityChainID() interfaces.IHash {
	return s.IdentityChainID
}

func (s StateMock) IsRunLeader() bool {
	return s.RunLeader
}

func (s StateMock) GetEventService() LiveFeedService {
	return s.Service
}

func (s StateMock) GetPubRegistry() pubsub.IPubRegistry {
	return s.PubRegistry
}
