package rag

import (
	"context"

	config_types "milestones-esp32-server-golang/internal/domain/config/types"
)

// Searcher triển khai việc truy vấn cơ sở tri thức theo từng provider.
type Searcher interface {
	Search(
		ctx context.Context,
		query string,
		topK int,
		knowledgeBases []config_types.KnowledgeBaseRef,
		providerConfig map[string]interface{},
	) ([]config_types.KnowledgeSearchHit, error)
}