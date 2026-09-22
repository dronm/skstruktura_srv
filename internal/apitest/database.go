//go:build integration

package apitest

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

const integrationDBOperationTimeout = 5 * time.Second

func deleteMaterialRequestFixture(t *testing.T, id int, suffix string) {
	t.Helper()

	connectionString := strings.TrimSpace(os.Getenv("API_TEST_DB_CONN"))
	if connectionString == "" {
		t.Fatal("API_TEST_DB_CONN is required to clean up a submitted material request fixture")
	}

	ctx, cancel := context.WithTimeout(context.Background(), integrationDBOperationTimeout)
	defer cancel()

	connection, err := pgx.Connect(ctx, connectionString)
	if err != nil {
		t.Fatalf("connect to integration test database: %v", err)
	}
	defer func() {
		closeCtx, closeCancel := context.WithTimeout(context.Background(), integrationDBOperationTimeout)
		defer closeCancel()
		if err := connection.Close(closeCtx); err != nil {
			t.Errorf("close integration test database connection: %v", err)
		}
	}()

	commandTag, err := connection.Exec(ctx, `
		DELETE FROM public.material_requests AS request
		USING
			public.construction_sites AS construction_site,
			public.users AS construction_manager
		WHERE request.id = $1
			AND construction_site.id = request.construction_site_id
			AND construction_site.name = $2
			AND construction_manager.id = request.construction_manager_id
			AND construction_manager.name = $3
	`, id, "request-site-"+suffix, "request-manager-"+suffix)
	if err != nil {
		t.Fatalf("delete material request fixture %d: %v", id, err)
	}
	if commandTag.RowsAffected() != 1 {
		t.Fatalf(
			"delete material request fixture %d affected %d rows, want 1",
			id,
			commandTag.RowsAffected(),
		)
	}
}
