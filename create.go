package dm8

import (
	"bytes"
	"database/sql"
	"reflect"

	"gitee.com/encircles/gorm-dm8/clauses"
	"github.com/thoas/go-funk"
	"gorm.io/gorm"
	"gorm.io/gorm/callbacks"
	"gorm.io/gorm/clause"
	gormSchema "gorm.io/gorm/schema"
)

func Create(db *gorm.DB) {

	// 可以获取表空间和模式名
	namer := db.NamingStrategy.(Namer)

	stmt := db.Statement
	schema := stmt.Schema
	boundVars := make(map[string]int)

	if stmt == nil || schema == nil {
		return
	}

	hasDefaultValues := len(schema.FieldsWithDefaultDBValue) > 0

	if !stmt.Unscoped {
		for _, c := range schema.CreateClauses {
			stmt.AddClause(c)
		}
	}

	if stmt.SQL.String() == "" {
		values := callbacks.ConvertToCreateValues(stmt)
		onConflict, hasConflict := stmt.Clauses["ON CONFLICT"].Expression.(clause.OnConflict)
		// are all columns in value the primary fields in schema only?
		if hasConflict && funk.Contains(
			funk.Map(values.Columns, func(c clause.Column) string { return c.Name }),
			funk.Map(schema.PrimaryFields, func(field *gormSchema.Field) string { return field.DBName }),
		) {
			stmt.AddClauseIfNotExists(clauses.Merge{
				Using: []clause.Interface{
					clause.Select{
						Columns: funk.Map(values.Columns, func(column clause.Column) clause.Column {
							// HACK: I can not come up with a better alternative for now
							// I want to add a value to the list of variable and then capture the bind variable position as well
							buf := bytes.NewBufferString("")
							stmt.Vars = append(stmt.Vars, values.Values[0][funk.IndexOf(values.Columns, column)])
							stmt.BindVarTo(buf, stmt, nil)

							column.Alias = column.Name
							// then the captured bind var will be the name
							column.Name = buf.String()
							return column
						}).([]clause.Column),
					},
					clause.From{
						Tables: []clause.Table{{Name: db.Dialector.(Dialector).DummyTableName()}},
					},
				},
				On: funk.Map(schema.PrimaryFields, func(field *gormSchema.Field) clause.Expression {
					return clause.Eq{
						Column: clause.Column{Table: stmt.Table, Name: field.DBName},
						Value:  clause.Column{Table: clauses.MergeDefaultExcludeName(), Name: field.DBName},
					}
				}).([]clause.Expression),
			})
			stmt.AddClauseIfNotExists(clauses.WhenMatched{Set: onConflict.DoUpdates})
			stmt.AddClauseIfNotExists(clauses.WhenNotMatched{Values: values})

			stmt.Build("MERGE", "WHEN MATCHED", "WHEN NOT MATCHED")
		} else {
			// stmt.AddClauseIfNotExists(clause.Insert{Table: clause.Table{Name: stmt.Table}})
			stmt.AddClauseIfNotExists(clause.Insert{Table: clause.Table{Name: namer.DmSchemaName + "." + stmt.Table}})
			stmt.AddClause(clause.Values{Columns: values.Columns, Values: [][]interface{}{values.Values[0]}})
			if hasDefaultValues {
				// stmt.AddClauseIfNotExists(clause.Returning{
				// 	Columns: funk.Map(schema.FieldsWithDefaultDBValue, func(field *gormSchema.Field) clause.Column {
				// 		return clause.Column{Name: field.DBName}
				// 	}).([]clause.Column),
				// })

				// stmt.AddClauseIfNotExists(clauses.ReturningInto{
				// 	Variables: funk.Map(schema.FieldsWithDefaultDBValue, func(field *gormSchema.Field) clause.Column {
				// 		return clause.Column{Name: field.DBName}
				// 	}).([]clause.Column),
				// })
			}
			stmt.Build("INSERT", "VALUES", "RETURNING")
			// stmt.Build("INSERT", "VALUES")
			if hasDefaultValues {
				// stmt.WriteString(" INTO ")
				// for _, field := range schema.FieldsWithDefaultDBValue {
				// 	// if idx > 0 {
				// 	// 	stmt.WriteByte(',')
				// 	// }
				//
				// 	out := sql.Named(field.DBName, sql.Out{Dest: reflect.New(field.FieldType).Interface()})
				// 	// out := sql.NamedArg{Name: field.DBName}
				// 	boundVars[field.Name] = len(stmt.Vars)
				// 	// stmt.AddVar(stmt, sql.Out{Dest: reflect.New(field.FieldType).Interface()})
				// 	stmt.AddVar(stmt, out)
				// }
			}
		}

		if !db.DryRun {
			for idx, vals := range values.Values {
				// HACK HACK: replace values one by one, assuming its value layout will be the same all the time, i.e. aligned
				for idx, val := range vals {
					switch v := val.(type) {
					case bool:
						if v {
							val = 1
						} else {
							val = 0
						}
					}

					stmt.Vars[idx] = val
				}
				// stmt.SQL.WriteString(fmt.Sprintf(" STORAGE(ON \"%s\", CLUSTERBTR)", namer.TableSpaceName))
				switch result, err := stmt.ConnPool.ExecContext(stmt.Context, stmt.SQL.String(), stmt.Vars...); err {
				case nil: // success
					db.RowsAffected, _ = result.RowsAffected()

					if db.RowsAffected != 0 {

						insertID, err := result.LastInsertId()
						insertOk := err == nil && insertID > 0
						if !insertOk {
							db.AddError(err)
							return
						}

						insertTo := stmt.ReflectValue
						switch insertTo.Kind() {
						case reflect.Slice, reflect.Array:
							insertTo = insertTo.Index(idx)
						case reflect.Struct:
							_, isZero := stmt.Schema.PrioritizedPrimaryField.ValueOf(db.Statement.ReflectValue)
							if isZero {
								stmt.Schema.PrioritizedPrimaryField.Set(db.Statement.ReflectValue, insertID)
							}
						}

						if hasDefaultValues {
							// bind returning value back to reflected value in the respective fields
							funk.ForEach(
								funk.Filter(schema.FieldsWithDefaultDBValue, func(field *gormSchema.Field) bool {
									name := field.Name
									// out := sql.Named(field.DBName, insertID)
									out := sql.Named(field.DBName, sql.Out{Dest: insertID})
									boundVars[field.Name] = len(stmt.Vars)
									stmt.AddVar(stmt, out)

									return funk.Contains(boundVars, name)
								}),
								func(field *gormSchema.Field) {

									kind := insertTo.Kind()
									switch kind {
									case reflect.Struct:
										if err = field.Set(insertTo, stmt.Vars[boundVars[field.Name]].(sql.Out).Dest); err != nil {
											db.AddError(err)
										}
									case reflect.Map:
										// todo 设置id的值
									}
								},
							)
						}
					}

				default: // failure
					db.AddError(err)
				}
			}
		}
	}
}
