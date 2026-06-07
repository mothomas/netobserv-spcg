package graphdb

import (
	"context"
	"fmt"

	neo4j "github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// RunWrite executes a managed write transaction using the store driver.
func (s *Store) RunWrite(ctx context.Context, fn func(tx neo4j.ManagedTransaction) error) error {
	if !s.Enabled() {
		return nil
	}
	sess := s.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer sess.Close(ctx)
	_, err := sess.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		return nil, fn(tx)
	})
	return err
}

// RunRead executes a managed read transaction using the store driver.
func (s *Store) RunRead(ctx context.Context, fn func(tx neo4j.ManagedTransaction) (any, error)) (any, error) {
	if !s.Enabled() {
		return nil, fmt.Errorf("neo4j graph store is not configured")
	}
	sess := s.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer sess.Close(ctx)
	return sess.ExecuteRead(ctx, fn)
}
