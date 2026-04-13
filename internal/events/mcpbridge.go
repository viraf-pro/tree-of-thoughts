package events

import (
	"fmt"
	"log"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// StartMCPBridge subscribes to the global bus and forwards events
// as MCP resource-updated notifications. Non-blocking — runs in a goroutine.
func StartMCPBridge(s *server.MCPServer) {
	id, ch := Get().Subscribe()
	go func() {
		defer Get().Unsubscribe(id)
		for evt := range ch {
			for _, uri := range affectedURIs(evt) {
				s.SendNotificationToAllClients(
					mcp.MethodNotificationResourceUpdated,
					map[string]any{"uri": uri},
				)
			}
			log.Printf("event: %s tree=%s node=%s", evt.Type, evt.TreeID, evt.NodeID)
		}
	}()
}

// affectedURIs maps an event to the resource URIs that changed.
func affectedURIs(e Event) []string {
	var uris []string

	// Always notify the tree list if a tree event
	switch e.Type {
	case TreeCreated, TreeStatusChanged, TreeAutoPaused:
		uris = append(uris, "tot://trees")
	}

	// Tree-specific resources
	if e.TreeID != "" {
		uris = append(uris, fmt.Sprintf("tot://tree/%s", e.TreeID))

		switch e.Type {
		case ThoughtAdded, SubtreePruned:
			uris = append(uris, fmt.Sprintf("tot://tree/%s/frontier", e.TreeID))
		case TreeStatusChanged, TreeAutoPaused:
			uris = append(uris, fmt.Sprintf("tot://tree/%s/status", e.TreeID))
		case ExperimentPrepared, ExperimentCompleted, ExperimentFailed:
			uris = append(uris, fmt.Sprintf("tot://tree/%s/experiments", e.TreeID))
		}
	}

	// Solution resources
	switch e.Type {
	case SolutionStored, SolutionCompacted, SolutionLinked, URLIngested:
		uris = append(uris, "tot://solutions")
		if sid, ok := e.Payload["solutionId"].(string); ok {
			uris = append(uris, fmt.Sprintf("tot://solution/%s", sid))
		}
	}

	// Tree links affect both trees
	if e.Type == TreeLinked {
		if tgt, ok := e.Payload["targetTree"].(string); ok {
			uris = append(uris, fmt.Sprintf("tot://tree/%s", tgt))
		}
	}

	return uris
}
