package clauses

import (
	"gorm.io/gorm/clause"
)

type ReturningInto struct {
	Variables []clause.Column
	Into      []*clause.Values
}

// Name where clause name
func (returning ReturningInto) Name() string {
	return "RETURNING"
}

// Build build where clause
func (returning ReturningInto) Build(builder clause.Builder) {
	if len(returning.Variables) > 0 {
		for idx, column := range returning.Variables {
			if idx > 0 {
				builder.WriteByte(',')
			}

			builder.WriteQuoted(column)
		}
	} else {
		builder.WriteByte('*')
	}
}

// MergeClause merge order by clauses
func (returning ReturningInto) MergeClause(clause *clause.Clause) {
	if v, ok := clause.Expression.(ReturningInto); ok {
		returning.Variables = append(v.Variables, returning.Variables...)
	}

	clause.Expression = returning
}
