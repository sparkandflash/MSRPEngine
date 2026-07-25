package contextManager

import (
	"context"
	"fmt"
	"msrpe-vron-go/src/utils"

	"github.com/philippgille/chromem-go"
)

// ContextIndexManager handles the persistent vector database operations.
type ContextIndexManager struct {
	Client   *chromem.DB
	Embedder *LocalEmbedder
}

// NewContextIndexManager initializes chromem-go and ensures the collections exist.
func NewContextIndexManager() (*ContextIndexManager, error) {
	// Initialize LocalEmbedder strictly for safe injection
	embedder := NewLocalEmbedder()

	// Initialize chromem-go client targeting the local disk
	client, err := chromem.NewPersistentDB(ChromemDB, false)
	if err != nil {
		return nil, fmt.Errorf("failed to init chromem-go DB: %v", err)
	}

	// Create or fetch the episodes collection (injecting the embedder to avoid panic)
	_, err = client.GetOrCreateCollection("episodes", nil, embedder.AsChromemEmbeddingFunc())
	if err != nil {
		return nil, fmt.Errorf("failed to create episodes collection: %v", err)
	}

	utils.LogDebug("Chromem-Go initialized at %s", ChromemDB)

	return &ContextIndexManager{
		Client:   client,
		Embedder: embedder,
	}, nil
}

// IndexEpisode safely injects an episode node into the vector database.
func (cim *ContextIndexManager) IndexEpisode(ep Episode) error {
	col := cim.Client.GetCollection("episodes", cim.Embedder.AsChromemEmbeddingFunc())
	if col == nil {
		return fmt.Errorf("failed to retrieve episodes collection")
	}

	// Convert Episode struct into a chromem Document
	doc := chromem.Document{
		ID:      ep.ID,
		Content: ep.Content,
		Metadata: map[string]string{
			"type":      ep.Type,
			"timestamp": fmt.Sprintf("%d", ep.Timestamp),
			"weight":    fmt.Sprintf("%d", ep.Weight),
		},
	}

	// Insert into DB. The embedder is automatically called here.
	err := col.AddDocument(context.Background(), doc)
	if err != nil {
		return err
	}

	utils.LogDebug("Indexed Episode into Chromem-Go | ID: %s", ep.ID)
	return nil
}
