package main

import (
	"sync"
)

// state is a collection of global variables representing the current state of
// minimega. We embed things where possible so that we can simplify calling
// unambigious functions -- for example, mm.FindVM instead of mm.vms.FindVM.
// There is one state per namespace.
type state struct {
	Name string

	VMs // embed
}

var stateLock sync.Mutex

// mm is the current state, set by changing namespaces
var mm *state = NewState("")

var savedStates = map[string]*state{
	"": mm,
}

func NewState(name string) *state {
	return &state{
		Name: name,
		VMs:  VMs{},
	}
}
