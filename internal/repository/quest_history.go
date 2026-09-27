package repository

import (
	"context"
	"fmt"

	"ntdkhiem/ppbudget-go/internal/domain"
)

// ListProgressEvents returns the user's action history across every action,
// newest first: the rows a progress timeline is made of.
//
// Only movements across the completion line are returned. "generated" marks
// an action first appearing on the plan, which is bookkeeping rather than
// progress -- the first reading of a plan writes one for nearly every action.
func (r *Repository) ListProgressEvents(ctx context.Context, userID string, limit int) ([]domain.QuestEvent, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, catalog_key, event, source, COALESCE(note, ''), created_at
		 FROM wealth_quest_events
		 WHERE user_id = $1 AND event IN ('completed', 'uncompleted', 'skipped', 'unskipped')
		 ORDER BY created_at DESC, id
		 LIMIT $2`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list progress events: %w", err)
	}
	defer rows.Close()

	out := []domain.QuestEvent{}
	for rows.Next() {
		var e domain.QuestEvent
		if err := rows.Scan(&e.ID, &e.CatalogKey, &e.Event, &e.Source, &e.Note, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
