// Start fileheader template
// Code generated by go generate; DO NOT EDIT.
// This file was generated by FactomGenerate robots

// Start Generated Code

package generated

import (
	. "github.com/FactomProject/factomd/common/pubsubtypes"
	. "github.com/FactomProject/factomd/pubsub/publishers"
)

// End fileheader template

// Start publisher generated go code

// Publish_Base_IMsg publisher has the basic necessary function implementations.
type Publish_Base_IMsg_type struct {
	*Base
}

// Receive the object of type and call the generic so the compiler can check the passed in type
func (p *Publish_Base_IMsg_type) Write(o IMsg) {
	p.Base.Write(o)
}

func Publish_Base_IMsg(p *Base) *Publish_Base_IMsg_type {
	return &Publish_Base_IMsg_type{p}
}

// End publisher generated go code
//
// Start filetail template
// Code generated by go generate; DO NOT EDIT.
// End filetail template
// End Generated Code
