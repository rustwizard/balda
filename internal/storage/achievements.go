package storage

import (
	"context"
	"fmt"

	"github.com/rustwizard/balda/internal/achievements"
)

// LoadAchievementDefinitions returns all achievement definitions from the database.
func (b *Balda) LoadAchievementDefinitions(ctx context.Context) ([]achievements.Definition, error) {
	ctx, cancel := context.WithTimeout(ctx, b.t)
	defer cancel()

	rows, err := b.db.Query(ctx,
		`SELECT id, name, description, COALESCE(icon_url, ''), condition_type, operator, threshold, bit_position
		 FROM achievements
		 ORDER BY bit_position`,
	)
	if err != nil {
		return nil, fmt.Errorf("load achievement definitions: %w", err)
	}
	defer rows.Close()

	out := make([]achievements.Definition, 0)
	for rows.Next() {
		var d achievements.Definition
		if err := rows.Scan(&d.ID, &d.Name, &d.Description, &d.IconURL, &d.ConditionType, &d.Operator, &d.Threshold, &d.BitPosition); err != nil {
			return nil, fmt.Errorf("load achievement definitions scan: %w", err)
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("load achievement definitions rows: %w", err)
	}
	return out, nil
}
