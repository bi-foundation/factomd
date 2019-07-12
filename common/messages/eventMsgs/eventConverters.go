package eventMessages

import (
	"github.com/FactomProject/factomd/common/interfaces"
	"github.com/FactomProject/factomd/common/messages"
)

func (*AnchoredEvent) ConvertFrom(dbStateMessage messages.DBStateMsg) *AnchoredEvent {
	event := new(AnchoredEvent)
	event.DirectoryBlock = convertFromDirBlock(dbStateMessage.DirectoryBlock)
	return event
}

func convertFromDirBlock(block interfaces.IDirectoryBlock) *DirectoryBlock {
	result := new(DirectoryBlock)
	result.Header = convertFromDirHeader(block.GetHeader())
	return result
}

func convertFromDirHeader(header interfaces.IDirectoryBlockHeader) *DirectoryBlockHeader {
	result := new(DirectoryBlockHeader)

	return result
}
