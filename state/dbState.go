// Copyright 2015 FactomProject Authors. All rights reserved.
// Use of this source code is governed by the MIT license
// that can be found in the LICENSE file.

package state

import (
	"encoding/hex"
	"fmt"
	"github.com/FactomProject/factomd/common/directoryBlock/dbInfo"
	"github.com/FactomProject/factomd/common/interfaces"
	"github.com/FactomProject/factomd/common/messages"
	"github.com/FactomProject/factomd/log"
	"time"
)

var _ = hex.EncodeToString
var _ = fmt.Print
var _ = time.Now()
var _ = log.Print

type DBState struct {
	isNew            bool
	DirectoryBlock   interfaces.IDirectoryBlock
	AdminBlock       interfaces.IAdminBlock
	FactoidBlock     interfaces.IFBlock
	EntryCreditBlock interfaces.IEntryCreditBlock
}

type DBStateList struct {
	last     interfaces.Timestamp
	state    *State
	base     uint32
	complete uint32
	DBStates []*DBState
}

const secondsBetweenTests = 2

// Once a second at most, we check to see if we need to pull down some blocks to catch up.
func (list *DBStateList) Catchup() {
	
	// We only check if we need updates once every so often.
	now := list.state.GetTimestamp()
	if(now/1000 - list.last/1000 < secondsBetweenTests) {
		return
	}
	list.last = now
fmt.Println("Check")	
	begin := -1 
	end := -1
	
	for i,v := range list.DBStates {
		if v == nil && begin < 0 {
			begin = i
		}
		if v == nil {
			end = i
		}
	}
	
	plHeight := list.state.ProcessLists.GetDBHeight()
	dbsHeight := list.base + uint32(len(list.DBStates))
	if plHeight - dbsHeight > 2 {
		msg := messages.NewDBStateMissing(list.state,uint32(plHeight-1),uint32(plHeight-1))
		if msg != nil { list.state.NetworkOutMsgQueue() <- msg }
	}
		
	if begin >= 0 {
		begin += int(list.base)
		end += int(list.base)
		msg := messages.NewDBStateMissing(list.state,uint32(begin),uint32(end))
		if msg != nil { list.state.NetworkOutMsgQueue() <- msg }
	}
}


func (list *DBStateList) GetDBHeight() uint32 {
	if list == nil { return 0 }
	if db := list.Last(); db == nil {
		return 0
	}else{
		return db.DirectoryBlock.GetHeader().GetDBHeight()
	}
}

func (list *DBStateList) Length() int {
	return len(list.DBStates)
}

func (list *DBStateList) Last() *DBState {
	last := len(list.DBStates)
	if last == 0 || last < int(list.complete) {
		return nil
	}
	return list.DBStates[last-1]
}

func (list *DBStateList) Put(dbstate *DBState) {

	dblk := dbstate.DirectoryBlock
	dbheight := dblk.GetHeader().GetDBHeight()
	
	cnt := len(list.DBStates)
	if cnt > 2 && int(list.complete) == len(list.DBStates) {
		list.DBStates = list.DBStates[cnt-2:]
		list.base = list.base + uint32(cnt) - 2
		list.complete = list.complete - uint32(cnt) + 2
	}
	index := int(dbheight) - int(list.base)
	
	// If we have already processed this state, ignore it.
	if index < int(list.complete) {
		return
	}
	
	// make room for this entry.
	for len(list.DBStates) <= index {
		list.DBStates = append(list.DBStates, nil)
	}
	
	list.DBStates[index] = dbstate

	hash, err := dbstate.AdminBlock.GetKeyMR()
	if err != nil {
		panic(err)
	}
	dbstate.DirectoryBlock.GetDBEntries()[0].SetKeyMR(hash)
	hash, err = dbstate.EntryCreditBlock.Hash()
	if err != nil {
		panic(err)
	}
	dbstate.DirectoryBlock.GetDBEntries()[1].SetKeyMR(hash)
	hash = dbstate.FactoidBlock.GetHash()
	dbstate.DirectoryBlock.GetDBEntries()[2].SetKeyMR(hash)
	
	if dbheight >= list.state.LDBHeight {
		list.state.LDBHeight = dbheight+1
		list.state.LastAck = nil
	}
}

func (list *DBStateList) Get(height uint32) *DBState {
	i := int(height) - int(list.base)
	if i < 0 || i >= len(list.DBStates) {
		return nil
	}
	return list.DBStates[i]
}

func (list *DBStateList) Process() {
		
	list.Catchup()
	
	for int(list.complete) < len(list.DBStates) {
		d := list.DBStates[list.complete]

		if d == nil {
			return
		}
		
	
		if d.isNew  {
			// Make sure the directory block is properly synced up with the prior block, if there
			// is one.
			
			dblk,_ := list.state.GetDB().FetchDBlockByHeight(d.DirectoryBlock.GetHeader().GetDBHeight())
			if dblk == nil {
				
				if list.complete > 0 {
					p := list.DBStates[list.complete-1]
					d.DirectoryBlock.GetHeader().SetPrevFullHash(p.DirectoryBlock.GetHeader().GetFullHash())
					d.DirectoryBlock.GetHeader().SetPrevKeyMR(p.DirectoryBlock.GetKeyMR())
				}
				
				if err := list.state.GetDB().SaveDirectoryBlockHead(d.DirectoryBlock); err != nil {
					panic(err.Error())
				}
				if err := list.state.GetDB().ProcessABlockBatch(d.AdminBlock); err != nil {
					panic(err.Error())
				}
				if err := list.state.GetDB().ProcessFBlockBatch(d.FactoidBlock); err != nil {
					panic(err.Error())
				}
				if err := list.state.GetDB().ProcessECBlockBatch(d.EntryCreditBlock); err != nil {
					panic(err.Error())
				}
			}
			list.state.GetAnchor().UpdateDirBlockInfoMap(dbInfo.NewDirBlockInfoFromDirBlock(d.DirectoryBlock))
			
		}
		fs := list.state.GetFactoidState()
		fs.AddTransactionBlock(d.FactoidBlock)
		fs.AddECBlock(d.EntryCreditBlock)
		fs.ProcessEndOfBlock(list.state)
		
		list.complete++
		if list.state.LDBHeight < list.complete+list.base {
			list.state.LDBHeight = list.complete + list.base
		}

	}
}

func (list *DBStateList) NewDBState(isNew bool,
	DirectoryBlock interfaces.IDirectoryBlock,
	AdminBlock interfaces.IAdminBlock,
	FactoidBlock interfaces.IFBlock,
	EntryCreditBlock interfaces.IEntryCreditBlock) *DBState {
		
	dbstate := new(DBState)

	dbstate.isNew = isNew
	dbstate.DirectoryBlock = DirectoryBlock
	dbstate.AdminBlock = AdminBlock
	dbstate.FactoidBlock = FactoidBlock
	dbstate.EntryCreditBlock = EntryCreditBlock

	list.Put(dbstate)
	return dbstate
}
