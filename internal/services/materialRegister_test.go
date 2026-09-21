package services

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/dronm/ds/v4"
)

type materialRegisterExecCall struct {
	query string
	args  []any
}

type materialRegisterFakeQuerier struct {
	execCalls []materialRegisterExecCall
	execErrAt int
}

func (q *materialRegisterFakeQuerier) Exec(
	_ context.Context,
	query string,
	args ...any,
) (ds.ExecResult, error) {
	q.execCalls = append(q.execCalls, materialRegisterExecCall{
		query: query,
		args:  append([]any(nil), args...),
	})
	if q.execErrAt > 0 && len(q.execCalls) == q.execErrAt {
		return nil, errors.New("exec failed")
	}

	return materialRegisterFakeExecResult{}, nil
}

func (q *materialRegisterFakeQuerier) Query(
	context.Context,
	string,
	...any,
) (ds.Rows, error) {
	return nil, errors.New("unexpected Query call")
}

func (q *materialRegisterFakeQuerier) QueryRow(
	context.Context,
	string,
	...any,
) ds.Row {
	return materialRegisterFakeRow{}
}

type materialRegisterFakeExecResult struct{}

func (materialRegisterFakeExecResult) RowsAffected() int64 {
	return 0
}

type materialRegisterFakeRow struct{}

func (materialRegisterFakeRow) Scan(...any) error {
	return errors.New("unexpected QueryRow call")
}

func TestRebuildMaterialRegisterActions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		recorderType string
		queryParts   []string
	}{
		{
			name:         "receipt adds stock",
			recorderType: materialReceiptRecorderType,
			queryParts: []string{
				"FROM public.material_receipts AS receipt",
				"COALESCE(item.construction_site_id, receipt.construction_site_id)",
				"item.quant",
			},
		},
		{
			name:         "consumption removes stock",
			recorderType: materialConsumptionRecorderType,
			queryParts: []string{
				"FROM public.material_consumptions AS consumption",
				"consumption.construction_site_id",
				"-item.quant",
			},
		},
		{
			name:         "transfer removes and adds stock",
			recorderType: materialTransferRecorderType,
			queryParts: []string{
				"transfer.source_construction_site_id",
				"transfer.destination_construction_site_id",
				"-item.quant AS quant",
				"item.quant AS quant",
				"UNION ALL",
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			querier := &materialRegisterFakeQuerier{}
			if err := rebuildMaterialRegisterActions(
				context.Background(),
				querier,
				test.recorderType,
				17,
			); err != nil {
				t.Fatalf("rebuildMaterialRegisterActions() error = %v", err)
			}

			if len(querier.execCalls) != 2 {
				t.Fatalf("Exec call count = %d, want 2", len(querier.execCalls))
			}
			if !strings.Contains(
				querier.execCalls[0].query,
				"public.ra_materials_remove_acts",
			) {
				t.Fatalf("first query does not remove old actions: %s", querier.execCalls[0].query)
			}

			writeCall := querier.execCalls[1]
			for _, queryPart := range append(
				[]string{
					"public.ra_materials_add_act",
				},
				test.queryParts...,
			) {
				if !strings.Contains(writeCall.query, queryPart) {
					t.Errorf("write query does not contain %q", queryPart)
				}
			}
			if strings.Contains(writeCall.query, "AT TIME ZONE") {
				t.Errorf("write query converts an already exact document timestamp: %s", writeCall.query)
			}
			if len(writeCall.args) != 2 ||
				writeCall.args[0] != test.recorderType ||
				writeCall.args[1] != 17 {
				t.Fatalf("write args = %#v", writeCall.args)
			}
		})
	}
}

func TestLockMaterialRecordersSortsAndDeduplicates(t *testing.T) {
	t.Parallel()

	querier := &materialRegisterFakeQuerier{}
	if err := lockMaterialRecorders(
		context.Background(),
		querier,
		materialReceiptRecorderType,
		5,
		2,
		5,
		0,
		3,
	); err != nil {
		t.Fatalf("lockMaterialRecorders() error = %v", err)
	}

	wantIDs := []any{2, 3, 5}
	if len(querier.execCalls) != len(wantIDs) {
		t.Fatalf("Exec call count = %d, want %d", len(querier.execCalls), len(wantIDs))
	}
	for index, wantID := range wantIDs {
		call := querier.execCalls[index]
		if len(call.args) != 2 || call.args[1] != wantID {
			t.Errorf("lock call %d args = %#v, want recorder id %v", index, call.args, wantID)
		}
	}
}

func TestRebuildMaterialRegisterActionsStopsAfterRemoveFailure(t *testing.T) {
	t.Parallel()

	querier := &materialRegisterFakeQuerier{execErrAt: 1}
	err := rebuildMaterialRegisterActions(
		context.Background(),
		querier,
		materialReceiptRecorderType,
		17,
	)
	if err == nil {
		t.Fatal("rebuildMaterialRegisterActions() error = nil, want error")
	}
	if len(querier.execCalls) != 1 {
		t.Fatalf("Exec call count = %d, want 1", len(querier.execCalls))
	}
}
