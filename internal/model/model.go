package model

import (
	"fmt"
	"net/netip"
)

// Typed identifiers keep fact fields readable while remaining comparable.
type (
	NodeID      string
	VRF         string
	InterfaceID string
	EdgePortID  string
)

const DefaultVRF VRF = "default"

type Node struct {
	ID NodeID
}

func (n Node) String() string {
	return fmt.Sprintf("Node{ID:%s}", n.ID)
}

type Interface struct {
	Node NodeID
	ID   InterfaceID
	VRF  VRF
}

func (i Interface) String() string {
	return fmt.Sprintf("Interface{Node:%s ID:%s VRF:%s}", i.Node, i.ID, i.VRF)
}

type Link struct {
	NodeA      NodeID
	InterfaceA InterfaceID
	NodeB      NodeID
	InterfaceB InterfaceID
}

func (l Link) String() string {
	return fmt.Sprintf(
		"Link{NodeA:%s InterfaceA:%s NodeB:%s InterfaceB:%s}",
		l.NodeA,
		l.InterfaceA,
		l.NodeB,
		l.InterfaceB,
	)
}

type EdgePort struct {
	ID        EdgePortID
	Node      NodeID
	Interface InterfaceID
	VRF       VRF
}

func (e EdgePort) String() string {
	return fmt.Sprintf(
		"EdgePort{ID:%s Node:%s Interface:%s VRF:%s}",
		e.ID,
		e.Node,
		e.Interface,
		e.VRF,
	)
}

type InterfaceAddress struct {
	Node      NodeID
	Interface InterfaceID
	VRF       VRF
	Prefix    netip.Prefix
}

func (a InterfaceAddress) String() string {
	return fmt.Sprintf(
		"InterfaceAddress{Node:%s Interface:%s VRF:%s Prefix:%s}",
		a.Node,
		a.Interface,
		a.VRF,
		a.Prefix,
	)
}

type InterfaceState struct {
	Node      NodeID
	Interface InterfaceID
	Up        bool
}

func (s InterfaceState) String() string {
	return fmt.Sprintf("InterfaceState{Node:%s Interface:%s Up:%t}", s.Node, s.Interface, s.Up)
}

type StaticRoute struct {
	Node    NodeID
	VRF     VRF
	Prefix  netip.Prefix
	Action  StaticRouteAction
	NextHop netip.Addr
}

func (r StaticRoute) String() string {
	return fmt.Sprintf(
		"StaticRoute{Node:%s VRF:%s Prefix:%s Action:%s NextHop:%s}",
		r.Node,
		r.VRF,
		r.Prefix,
		r.Action,
		r.NextHop,
	)
}

type StaticRouteAction string

const (
	StaticRouteActionNextHop StaticRouteAction = "next-hop"
	StaticRouteActionDrop    StaticRouteAction = "drop"
)

type ConnectedRoute struct {
	Node      NodeID
	VRF       VRF
	Prefix    netip.Prefix
	Interface InterfaceID
}

func (r ConnectedRoute) String() string {
	return fmt.Sprintf(
		"ConnectedRoute{Node:%s VRF:%s Prefix:%s Interface:%s}",
		r.Node,
		r.VRF,
		r.Prefix,
		r.Interface,
	)
}

type ForwardAction string

const (
	ForwardActionNextHop   ForwardAction = "next-hop"
	ForwardActionInterface ForwardAction = "interface"
	ForwardActionDrop      ForwardAction = "drop"
)

type ForwardingRule struct {
	Node      NodeID
	VRF       VRF
	Prefix    netip.Prefix
	Action    ForwardAction
	NextHop   netip.Addr
	Interface InterfaceID
}

func (r ForwardingRule) String() string {
	return fmt.Sprintf(
		"ForwardingRule{Node:%s VRF:%s Prefix:%s Action:%s NextHop:%s Interface:%s}",
		r.Node,
		r.VRF,
		r.Prefix,
		r.Action,
		r.NextHop,
		r.Interface,
	)
}

type Reach struct {
	Source EdgePortID
	Dest   EdgePortID
	VRF    VRF
	Prefix netip.Prefix
}

func (r Reach) String() string {
	return fmt.Sprintf(
		"Reach{Source:%s Dest:%s VRF:%s Prefix:%s}",
		r.Source,
		r.Dest,
		r.VRF,
		r.Prefix,
	)
}
