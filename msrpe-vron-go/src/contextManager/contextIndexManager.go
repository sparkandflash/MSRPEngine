package contextManager

import (
	"context"
	"fmt"
	"msrpe-vron-go/src/providers"
	"msrpe-vron-go/src/utils"

	"github.com/philippgille/chromem-go"
)

// ContextIndexManager handles the persistent vector database operations.
type ContextIndexManager struct {
	Client           *chromem.DB
	Embedder         *LocalEmbedder          // Safe mock embedder (always available)
	RealEmbedder     providers.EmbeddingProvider // Real API embedder (set after boot)
}

// NewContextIndexManager initializes chromem-go and ensures the collections exist.
func NewContextIndexManager() (*ContextIndexManager, error) {
	// Initialize real embedder from envconfig independently
	embedder := providers.NewEmbeddingProvider()

	// Initialize chromem-go client targeting the local disk
	client, err := chromem.NewPersistentDB(ChromemDB, false)
	if err != nil {
		return nil, fmt.Errorf("failed to init chromem-go DB: %v", err)
	}

	// Create or fetch the episodes collection
	embedFunc := func(ctx context.Context, text string) ([]float32, error) {
		return embedder.Embed(ctx, text)
	}
	_, err = client.GetOrCreateCollection("episodes", nil, embedFunc)
	if err != nil {
		return nil, fmt.Errorf("failed to create episodes collection: %v", err)
	}

	utils.LogDebug("Chromem-Go initialized at %s", ChromemDB)

	return &ContextIndexManager{
		Client:       client,
		RealEmbedder: embedder,
	}, nil
}

// IndexEpisode safely injects an episode node into the vector database.
func (cim *ContextIndexManager) IndexEpisode(ep Episode) error {
	embedFunc := func(ctx context.Context, text string) ([]float32, error) {
		return cim.RealEmbedder.Embed(ctx, text)
	}

	col := cim.Client.GetCollection("episodes", embedFunc)
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

// QueryEpisodes performs a nearest-neighbor semantic search in the vector DB.
// Returns up to maxResults episodes most relevant to the query string.
func (cim *ContextIndexManager) QueryEpisodes(query string, maxResults int) ([]Episode, error) {
	embedFunc := func(ctx context.Context, text string) ([]float32, error) {
		return cim.RealEmbedder.Embed(ctx, text)
	}

	col := cim.Client.GetCollection("episodes", embedFunc)
	if col == nil {
		return nil, fmt.Errorf("episodes collection not found")
	}

	results, err := col.Query(context.Background(), query, maxResults, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("chromem-go query failed: %w", err)
	}

	episodes := make([]Episode, 0, len(results))
	for _, doc := range results {
		weight := 0
		fmt.Sscanf(doc.Metadata["weight"], "%d", &weight)
		episodes = append(episodes, Episode{
			ID:      doc.ID,
			Type:    doc.Metadata["type"],
			Content: doc.Content,
			Weight:  weight,
		})
	}

	return episodes, nil
}
