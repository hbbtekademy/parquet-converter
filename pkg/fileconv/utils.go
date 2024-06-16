package fileconv

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hbbtekademy/go-fileconv/pkg/model"
)

func (c *fileconv) FlattenStructColumn(ctx context.Context, columnDesc *model.ColumnDesc) ([]*model.ColumnDesc, error) {
	if !columnDesc.ColType.IsStruct() {
		return nil, errors.New("column type not STRUCT")
	}

	tableName, err := c.createStructColTable(ctx, columnDesc)
	if err != nil {
		return nil, err
	}
	defer c.dropTable(ctx, tableName)

	tableDesc, err := c.GetTableDesc(ctx, fmt.Sprintf("SELECT C1.* FROM %s", tableName))
	if err != nil {
		return nil, err
	}

	columns := []*model.ColumnDesc{}
	for i := range tableDesc.ColumnDescs {
		if tableDesc.ColumnDescs[i].ColType.IsStruct() {
			cols, err := c.FlattenStructColumn(ctx, tableDesc.ColumnDescs[i])
			if err != nil {
				return nil, err
			}
			for i := range cols {
				col := &model.ColumnDesc{
					ColName: fmt.Sprintf("%s_%s", columnDesc.ColName, cols[i].ColName),
					ColType: cols[i].ColType,
				}
				columns = append(columns, col)
			}
			continue
		}

		col := &model.ColumnDesc{
			ColName: fmt.Sprintf("%s_%s", columnDesc.ColName, tableDesc.ColumnDescs[i].ColName),
			ColType: tableDesc.ColumnDescs[i].ColType,
		}
		columns = append(columns, col)
	}

	return columns, nil
}

func (c *fileconv) GetTableDesc(ctx context.Context, table string) (*model.TableDesc, error) {
	rows, err := c.db.QueryContext(ctx, fmt.Sprintf("SELECT COLUMN_NAME, COLUMN_TYPE FROM (DESCRIBE %s)", table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns := []*model.ColumnDesc{}
	for rows.Next() {
		var colName, colType string
		if err := rows.Scan(&colName, &colType); err != nil {
			return nil, err
		}
		columnDesc := &model.ColumnDesc{
			ColName: colName,
			ColType: model.ColumnType(colType),
		}
		columns = append(columns, columnDesc)
	}

	return &model.TableDesc{ColumnDescs: columns}, nil
}

func (c *fileconv) createStructColTable(ctx context.Context, columnDesc *model.ColumnDesc) (string, error) {
	tableName := fmt.Sprintf("%s_tmp_%d", columnDesc.ColName, time.Now().UnixMicro())
	_, err := c.db.ExecContext(ctx, fmt.Sprintf("CREATE TABLE %s (C1 %s)", tableName, columnDesc.ColType))
	if err != nil {
		return "", err
	}

	return tableName, nil
}

func (c *fileconv) dropTable(ctx context.Context, tableName string) error {
	_, err := c.db.ExecContext(ctx, fmt.Sprintf("DROP TABLE %s", tableName))
	return err
}

func (c *fileconv) getFlattenedTableSelect(ctx context.Context, tableName string) (string, error) {

	tableDesc, err := c.GetTableDesc(ctx, tableName)
	if err != nil {
		return "", fmt.Errorf("failed getting imported json table desc. error: %w", err)
	}

	flattenedColumns := []*model.ColumnDesc{}
	for i := range tableDesc.ColumnDescs {
		if tableDesc.ColumnDescs[i].ColType.IsStruct() {
			cols, err := c.FlattenStructColumn(ctx, tableDesc.ColumnDescs[i])
			if err != nil {
				return "", fmt.Errorf("failed flattening column: %s. error: %w",
					tableDesc.ColumnDescs[i].ColName, err)
			}
			flattenedColumns = append(flattenedColumns, cols...)
			continue
		}
		flattenedColumns = append(flattenedColumns, tableDesc.ColumnDescs[i])

	}

	unnestedCols, err := tableDesc.GetUnnestedColumns()
	if err != nil {
		return "", fmt.Errorf("failed getting unnested columns. error: %w", err)
	}

	unnestedTableSelect := fmt.Sprintf("SELECT %s FROM %s", unnestedCols, tableName)
	unnestedTableDesc, err := c.GetTableDesc(ctx, unnestedTableSelect)
	if err != nil {
		return "", fmt.Errorf("failed getting unnested table desc. error: %w", err)
	}

	if len(unnestedTableDesc.ColumnDescs) != len(flattenedColumns) {
		return "", fmt.Errorf("unnested table columns and flattened columns not matching. unnested table cols: %v, flattened cols: %v",
			unnestedTableDesc.ColumnDescs, flattenedColumns)
	}

	l := len(flattenedColumns)
	var sb strings.Builder

	sb.WriteString("SELECT ")
	for i := range flattenedColumns {
		sb.WriteString(fmt.Sprintf("%s AS %s",
			unnestedTableDesc.ColumnDescs[i].ColName,
			flattenedColumns[i].ColName))

		if i < l-1 {
			sb.WriteRune(',')
		}
	}

	sb.WriteString(fmt.Sprintf(" FROM (%s)", unnestedTableSelect))
	return sb.String(), nil
}
