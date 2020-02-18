package livefeed

import (
	"github.com/FactomProject/factomd/common/constants/runstate"
	"github.com/FactomProject/factomd/common/interfaces"
)

type StateEventServices interface {
	GetRunState() runstate.RunState
	GetIdentityChainID() interfaces.IHash
	GetRunLeader() bool
	GetLiveFeedService() LiveFeedService
	GetFactomNodeName() string
}
