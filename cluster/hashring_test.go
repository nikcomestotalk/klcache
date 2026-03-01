package cluster

import (
	"testing"
)

func TestHashRing_AddRemoveLocate(t *testing.T) {
	ring := NewHashRing()

	ring.AddMember("node1", "127.0.0.1:9001")
	ring.AddMember("node2", "127.0.0.1:9002")
	ring.AddMember("node3", "127.0.0.1:9003")

	node, addr, err := ring.GetOwner("my-cool-key")
	if err != nil {
		t.Fatalf("Expected no error locating key, got %v", err)
	}

	if node == "" || addr == "" {
		t.Fatalf("Expected valid node and addr, got node=%v addr=%v", node, addr)
	}

	// Make sure the same key maps to the same node
	node2, _, _ := ring.GetOwner("my-cool-key")
	if node != node2 {
		t.Fatalf("Consistent hashing failed, expected %s, got %s", node, node2)
	}

	// Remove the node that currently owns the key
	ring.RemoveMember(node)

	// The key should now map to a DIFFERENT node
	newNode, _, err := ring.GetOwner("my-cool-key")
	if err != nil {
		t.Fatalf("Expected no error locating key after removal, got %v", err)
	}

	if newNode == node {
		t.Fatalf("Expected owner to change after removal, but got the same node: %s", newNode)
	}
}
