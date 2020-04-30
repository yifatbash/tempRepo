package node

import (
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	bkeys "github.com/mosaicnetworks/babble/src/crypto/keys"
	"github.com/mosaicnetworks/babble/src/peers"
)

/*

Tests for dynamic membership protocol (adding/removing peers)

*/

func TestMonologue(t *testing.T) {
	keys, peers := initPeers(t, 1)

	genesisPeerSet := clonePeerSet(t, peers.Peers)

	nodes := initNodes(keys, peers, genesisPeerSet, 100000, 1000, 5, false, "inmem", 5*time.Millisecond, false, "", t)
	//defer drawGraphs(nodes, t)

	target := 50
	err := gossip(nodes, target, true)
	if err != nil {
		t.Fatalf("Fatal Error: %v", err)
	}

	checkGossip(nodes, 0, t)
}

func TestJoinRequest(t *testing.T) {

	keys, peerSet := initPeers(t, 4)

	genesisPeerSet := clonePeerSet(t, peerSet.Peers)

	nodes := initNodes(keys, peerSet, genesisPeerSet, 1000000, 1000, 5, false, "inmem", 5*time.Millisecond, false, "", t)
	defer shutdownNodes(nodes)
	//defer drawGraphs(nodes, t)

	target := 30
	err := gossip(nodes, target, false)
	if err != nil {
		t.Fatalf("Fatal Error: %v", err)
	}
	checkGossip(nodes, 0, t)

	key, _ := bkeys.GenerateECDSAKey()
	peer := peers.NewPeer(
		bkeys.PublicKeyHex(&key.PublicKey),
		fmt.Sprint("127.0.0.1:4242"),
		"monika",
	)

	newNode := newNode(peer, key, peerSet, genesisPeerSet, 1000, 1000, 5, false, "inmem", 5*time.Millisecond, false, "", t)
	defer newNode.Shutdown()

	err = newNode.join()
	if err != nil {
		t.Fatalf("Fatal Error: %v", err)
	}

	//Gossip some more
	secondTarget := target + 30
	err = bombardAndWait(nodes, secondTarget)
	if err != nil {
		t.Fatalf("Fatal Error: %v", err)
	}

	checkGossip(nodes, 0, t)
	checkPeerSets(nodes, t)
	verifyNewPeerSet(nodes, newNode.core.acceptedRound, 5, t)
}

func TestLeaveRequest(t *testing.T) {

	keys, peerSet := initPeers(t, 4)

	genesisPeerSet := clonePeerSet(t, peerSet.Peers)

	nodes := initNodes(keys, peerSet, genesisPeerSet, 1000000, 1000, 5, false, "inmem", 5*time.Millisecond, false, "", t)
	defer shutdownNodes(nodes)
	//defer drawGraphs(nodes, t)

	target := 30
	err := gossip(nodes, target, false)
	if err != nil {
		t.Fatalf("Fatal Error: %v", err)
	}
	checkGossip(nodes, 0, t)

	leavingNode := nodes[3]

	err = leavingNode.Leave()
	if err != nil {
		t.Fatalf("Fatal Error: %v", err)
	}

	//Gossip some more
	secondTarget := target + 50
	err = bombardAndWait(nodes[:3], secondTarget)
	if err != nil {
		t.Fatalf("Fatal Error: %v", err)
	}

	checkGossip(nodes[0:3], 0, t)
	checkPeerSets(nodes[0:3], t)
	verifyNewPeerSet(nodes[0:3], leavingNode.core.removedRound, 3, t)
}

func TestJoinFull(t *testing.T) {
	keys, peerSet := initPeers(t, 4)

	f := func(fastSync bool) {
		genesisPeerSet := clonePeerSet(t, peerSet.Peers)

		initialNodes := initNodes(keys, peerSet, genesisPeerSet, 1000000, 400, 5, fastSync, "inmem", 10*time.Millisecond, false, "", t)
		defer shutdownNodes(initialNodes)

		target := 30
		err := gossip(initialNodes, target, false)
		if err != nil {
			t.Fatalf("Fatal Error: %v", err)
		}
		checkGossip(initialNodes, 0, t)

		key, _ := bkeys.GenerateECDSAKey()
		peer := peers.NewPeer(
			bkeys.PublicKeyHex(&key.PublicKey),
			fmt.Sprint("127.0.0.1:4243"),
			"monika",
		)

		newNode := newNode(peer, key, peerSet, genesisPeerSet, 1000000, 400, 5, fastSync, "inmem", 10*time.Millisecond, false, "", t)
		defer newNode.Shutdown()

		newNode.RunAsync(true)

		nodes := append(initialNodes, newNode)

		//defer drawGraphs(nodes, t)

		//Gossip some more
		secondTarget := target + 50
		err = bombardAndWait(nodes, secondTarget)
		if err != nil {
			t.Fatalf("Fatal Error: %v", err)
		}

		start := newNode.core.hg.FirstConsensusRound
		checkGossip(nodes, *start, t)
		checkPeerSets(nodes, t)
		verifyNewPeerSet(nodes, newNode.core.acceptedRound, 5, t)
	}

	t.Run("FastSync enabled", func(t *testing.T) { f(true) })

	t.Run("FastSync disabled", func(t *testing.T) { f(false) })
}

// This verifies that a node does not suspend itself when it attempts to rejoin
// a network, with both types of stores. It also tests the stopping condition
// in hashgraph.updateAncestorFirstDescendants.
func TestRejoin(t *testing.T) {
	keys, peers := initPeers(t, 2)

	f := func(store string) {
		os.RemoveAll("test_data")
		os.Mkdir("test_data", os.ModeDir|0777)

		genesisPeerSet := clonePeerSet(t, peers.Peers)

		nodes := initNodes(keys, peers, genesisPeerSet, 50000, 50, 5, false, store, 10*time.Millisecond, false, "", t)
		//defer drawGraphs(nodes, t)
		defer shutdownNodes(nodes)

		// Start 2 nodes and let them create some blocks
		target := 50
		err := gossip(nodes, target, false)
		if err != nil {
			t.Fatal(err)
		}
		checkGossip(nodes, 0, t)

		// Stop node2
		leavingNode := nodes[1]
		err = leavingNode.Leave()
		if err != nil {
			t.Fatal(err)
		}

		// Let node1 create more blocks alone
		err = bombardAndWait(nodes[:1], 2*target)
		if err != nil {
			t.Fatal(err)
		}
		checkGossip(nodes[:1], 0, t)

		// Restart node2
		nodes[1] = recycleNode(leavingNode, t)
		nodes[1].RunAsync(true)

		// Let both nodes create more blocks
		err = bombardAndWait(nodes, 3*target)
		if err != nil {
			t.Fatalf("ERR: %s", err)
		}

		shutdownNodes(nodes)

		checkGossip(nodes, 0, t)
	}

	t.Run("InmemStore", func(t *testing.T) { f("inmem") })

	t.Run("BadgerStore", func(t *testing.T) { f("badger") })
}

func checkPeerSets(nodes []*Node, t *testing.T) {
	node0FP, err := nodes[0].core.hg.Store.GetAllPeerSets()
	if err != nil {
		t.Fatalf("Fatal Error: %v", err)
	}
	for i := range nodes[1:] {
		nodeiFP, err := nodes[i].core.hg.Store.GetAllPeerSets()
		if err != nil {
			t.Fatalf("Fatal Error: %v", err)
		}
		if !reflect.DeepEqual(node0FP, nodeiFP) {
			t.Logf("Node 0 PeerSets: %v", node0FP)
			t.Logf("Node %d PeerSets: %v", i, nodeiFP)
			t.Fatalf("PeerSets differ")
		}
	}
}

func verifyNewPeerSet(nodes []*Node, round int, expectedLength int, t *testing.T) {
	for i, node := range nodes {
		nodeFP, _ := node.core.hg.Store.GetAllPeerSets()

		nps, ok := nodeFP[round]
		if !ok {
			t.Fatalf("nodes[%d] PeerSets[%d] should not be empty", i, round)
		}

		if len(nps) != expectedLength {
			t.Fatalf("nodes[%d] PeerSets[%d] should contain %d peers, not %d", i, round, expectedLength, len(nps))
		}
	}
}
