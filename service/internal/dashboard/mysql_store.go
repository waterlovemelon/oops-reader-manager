package dashboard

import (
	"context"
	"database/sql"
	"fmt"
)

type MySQLStore struct {
	db *sql.DB
}

func NewMySQLStore(db *sql.DB) *MySQLStore {
	return &MySQLStore{db: db}
}

func (s *MySQLStore) Summary(ctx context.Context) (Summary, error) {
	var summary Summary
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&summary.TotalUsers); err != nil {
		return summary, fmt.Errorf("count users: %w", err)
	}
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM catalog_books WHERE status <> 'deleted'").Scan(&summary.TotalBooks); err != nil {
		return summary, fmt.Errorf("count books: %w", err)
	}
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM community_threads WHERE status <> 'deleted'").Scan(&summary.TotalThreads); err != nil {
		return summary, fmt.Errorf("count threads: %w", err)
	}
	var draftBooks, hiddenThreads int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM catalog_books WHERE status = 'draft'").Scan(&draftBooks); err != nil {
		return summary, fmt.Errorf("count draft books: %w", err)
	}
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM community_threads WHERE status = 'hidden'").Scan(&hiddenThreads); err != nil {
		return summary, fmt.Errorf("count hidden threads: %w", err)
	}
	summary.PendingReview = draftBooks + hiddenThreads
	return summary, nil
}
