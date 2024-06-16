package fileconv

import (
	"context"
	"testing"

	"github.com/hbbtekademy/go-fileconv/pkg/model"
)

func TestFlattenStructColumn(t *testing.T) {
	colDesc := &model.ColumnDesc{
		ColName: "a2",
		ColType: "STRUCT(b1 VARCHAR, b2 STRUCT(c1 BIGINT, c2 VARCHAR, c3 STRUCT(d1 BIGINT)), b3 STRUCT(d1 DOUBLE, d2 VARCHAR))",
	}

	conv, err := New(context.Background(), "")
	if err != nil {
		t.Fatalf("failed getting converter. error: %v", err)
	}

	cols, err := conv.FlattenStructColumn(context.Background(), colDesc)
	if err != nil {
		t.Fatalf("failed flattening struct col. error: %v", err)
	}

	t.Log(cols)
}
