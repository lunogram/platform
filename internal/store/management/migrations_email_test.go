package management

import (
	"context"
	"database/sql"
	"testing"

	"github.com/cloudproud/graceful"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/container"
	"github.com/lunogram/platform/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// versionBeforeEmailReconciliation is the last migration that predates
// 1787000000001_admin_email_lower_unique. Stopping here lets a test seed the
// duplicate-email state exactly as a customer database can hold it: the old
// case-SENSITIVE unique index is still in force, so 'A@x.com' and 'a@x.com'
// coexist.
const versionBeforeEmailReconciliation = 1781000000000

// migrateTo applies migrations up to and including version.
func migrateTo(t *testing.T, uri string, version uint) {
	t.Helper()

	source, err := iofs.New(migrations, "migrations")
	require.NoError(t, err)

	conn, err := sql.Open("pgx", uri)
	require.NoError(t, err)
	defer conn.Close()

	driver, err := pgx.WithInstance(conn, &pgx.Config{})
	require.NoError(t, err)

	migrator, err := migrate.NewWithInstance("iofs", source, "pgx", driver)
	require.NoError(t, err)

	require.NoError(t, migrator.Migrate(version))
}

// duplicateEmailFixture is one seeded admin in the pre-migration state. The ids
// and timestamps are fixed rather than generated so the expected keeper is a
// property of the data, not of whatever order the rows happened to be written
// in -- which is the entire point of the reconciliation's tiebreakers.
type duplicateEmailFixture struct {
	id          uuid.UUID
	email       string
	externalID  string
	createdAt   string
	memberships int
	projects    int
}

func adminUUID(t *testing.T, s string) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(s)
	require.NoError(t, err)
	return id
}

func duplicateEmailFixtures(t *testing.T) []duplicateEmailFixture {
	t.Helper()
	return []duplicateEmailFixture{
		// Organization membership outranks age: the younger row keeps the
		// address because it is the one people actually work in.
		{
			id: adminUUID(t, "aaaaaaaa-0000-0000-0000-000000000001"), email: "Dup@Example.com",
			externalID: "user_dup_keeper", createdAt: "2024-01-02T00:00:00Z", memberships: 2,
		},
		{
			id: adminUUID(t, "aaaaaaaa-0000-0000-0000-000000000002"), email: "dup@example.com",
			externalID: "user_dup_loser", createdAt: "2024-01-01T00:00:00Z",
		},
		// With memberships equal, project membership breaks the tie -- again
		// ahead of age.
		{
			id: adminUUID(t, "bbbbbbbb-0000-0000-0000-000000000001"), email: "Proj@Example.com",
			externalID: "user_proj_keeper", createdAt: "2024-05-02T00:00:00Z", projects: 1,
		},
		{
			id: adminUUID(t, "bbbbbbbb-0000-0000-0000-000000000002"), email: "proj@example.com",
			externalID: "user_proj_loser", createdAt: "2024-05-01T00:00:00Z",
		},
		// Nothing to separate these two but the final tiebreakers. Equal
		// created_at leaves the lowest id, which cannot depend on physical row
		// order -- exactly the property the old index failed to provide.
		{
			id: adminUUID(t, "cccccccc-0000-0000-0000-000000000001"), email: "Tie@Example.com",
			externalID: "user_tie_keeper", createdAt: "2024-03-01T00:00:00Z",
		},
		{
			id: adminUUID(t, "cccccccc-0000-0000-0000-000000000002"), email: "tie@example.com",
			externalID: "user_tie_loser", createdAt: "2024-03-01T00:00:00Z",
		},
		// An address nobody collides on must be left entirely alone.
		{
			id: adminUUID(t, "dddddddd-0000-0000-0000-000000000001"), email: "solo@example.com",
			externalID: "user_solo", createdAt: "2024-07-01T00:00:00Z", memberships: 1,
		},
	}
}

// expectedKeepers maps each colliding address to the admin the reconciliation
// must elect, and expectedQuarantined lists the rows it must set aside.
var (
	expectedKeepers = []string{
		"aaaaaaaa-0000-0000-0000-000000000001",
		"bbbbbbbb-0000-0000-0000-000000000001",
		"cccccccc-0000-0000-0000-000000000001",
		"dddddddd-0000-0000-0000-000000000001",
	}
	expectedQuarantined = []string{
		"aaaaaaaa-0000-0000-0000-000000000002",
		"bbbbbbbb-0000-0000-0000-000000000002",
		"cccccccc-0000-0000-0000-000000000002",
	}
)

// seedDuplicateEmails writes the fixtures in the given order. Raw SQL, because
// the store no longer knows how to write an external_id or a fixed id -- the
// point is to reproduce what is already on disk in a customer database.
func seedDuplicateEmails(t *testing.T, uri string, order []int) {
	t.Helper()

	conn, err := sql.Open("pgx", uri)
	require.NoError(t, err)
	defer conn.Close()

	ctx := context.Background()

	var orgID uuid.UUID
	require.NoError(t, conn.QueryRowContext(ctx,
		`INSERT INTO organizations (name) VALUES ('Seeded Organization') RETURNING id`).Scan(&orgID))

	var projectID uuid.UUID
	require.NoError(t, conn.QueryRowContext(ctx,
		`INSERT INTO projects (organization_id, name) VALUES ($1, 'Seeded Project') RETURNING id`, orgID).Scan(&projectID))

	var extraOrgID uuid.UUID
	require.NoError(t, conn.QueryRowContext(ctx,
		`INSERT INTO organizations (name) VALUES ('Extra Organization') RETURNING id`).Scan(&extraOrgID))

	fixtures := duplicateEmailFixtures(t)
	for _, i := range order {
		f := fixtures[i]

		_, err := conn.ExecContext(ctx, `
			INSERT INTO admins (id, organization_id, email, external_id, role, created_at)
			VALUES ($1, $2, $3, $4, 'owner', $5)`,
			f.id, orgID, f.email, f.externalID, f.createdAt)
		require.NoError(t, err)

		orgs := []uuid.UUID{orgID, extraOrgID}
		for m := 0; m < f.memberships; m++ {
			_, err := conn.ExecContext(ctx,
				`INSERT INTO organization_members (organization_id, admin_id, role) VALUES ($1, $2, 'owner')`,
				orgs[m], f.id)
			require.NoError(t, err)
		}

		for p := 0; p < f.projects; p++ {
			_, err := conn.ExecContext(ctx,
				`INSERT INTO project_admins (project_id, admin_id, role) VALUES ($1, $2, 'admin')`,
				projectID, f.id)
			require.NoError(t, err)
		}
	}
}

// reconcileDuplicateEmails seeds a fresh schema in the given row order, runs the
// remaining migrations, and returns a connected store.
func reconcileDuplicateEmails(t *testing.T, schema string, order []int) (*State, store.DB) {
	t.Helper()

	uri := container.CreateSchema(t, container.RunPostgreSQL(t), schema)
	migrateTo(t, uri, versionBeforeEmailReconciliation)
	seedDuplicateEmails(t, uri, order)
	require.NoError(t, Migrate(uri))

	conn, err := store.Connect(graceful.NewContext(t.Context()), zaptest.NewLogger(t), uri)
	require.NoError(t, err)
	return NewState(conn), conn
}

// TestDuplicateEmailReconciliation is the highest-risk piece of this change: the
// migration runs unattended against customer databases, so it must never fail
// and must never merge two accounts. It elects one canonical row per colliding
// address and quarantines the rest, keeping every admin able to log in through
// admin_identities.
func TestDuplicateEmailReconciliation(t *testing.T) {
	t.Parallel()

	db, conn := reconcileDuplicateEmails(t, "reconcile_primary", []int{0, 1, 2, 3, 4, 5, 6})
	ctx := context.Background()

	t.Run("elects the ranked keeper for every colliding address", func(t *testing.T) {
		for _, id := range expectedKeepers {
			admin, err := db.GetAdmin(ctx, adminUUID(t, id))
			require.NoError(t, err)
			assert.NotNil(t, admin)

			var quarantined bool
			require.NoError(t, conn.GetContext(ctx, &quarantined,
				`SELECT email_conflict_at IS NOT NULL FROM admins WHERE id = $1`, admin.ID))
			assert.False(t, quarantined, "admin %s should have kept the address", id)
		}
	})

	t.Run("quarantines the surplus rows without deleting or rewriting them", func(t *testing.T) {
		for _, id := range expectedQuarantined {
			admin, err := db.GetAdmin(ctx, adminUUID(t, id))
			require.NoError(t, err, "a quarantined admin must still exist")

			var quarantined bool
			require.NoError(t, conn.GetContext(ctx, &quarantined,
				`SELECT email_conflict_at IS NOT NULL FROM admins WHERE id = $1`, admin.ID))
			assert.True(t, quarantined, "admin %s should have been quarantined", id)
			assert.NotEmpty(t, admin.Email, "a quarantined admin keeps its real address")
		}
	})

	t.Run("the email join returns the keeper", func(t *testing.T) {
		admin, err := db.GetAdminByEmail(ctx, "dup@example.com")
		require.NoError(t, err)
		assert.Equal(t, adminUUID(t, "aaaaaaaa-0000-0000-0000-000000000001"), admin.ID)

		// Case-insensitively too: that is what the new index is for.
		admin, err = db.GetAdminByEmail(ctx, "DUP@EXAMPLE.COM")
		require.NoError(t, err)
		assert.Equal(t, adminUUID(t, "aaaaaaaa-0000-0000-0000-000000000001"), admin.ID)
	})

	t.Run("linking an identity onto a contested address fails closed", func(t *testing.T) {
		_, err := db.ResolveAdminByEmail(ctx, "dup@example.com")
		require.ErrorIs(t, err, ErrAmbiguousEmail,
			"a contested address must not be resolved by guessing which account was meant")

		var ambiguous *AmbiguousEmailError
		require.ErrorAs(t, err, &ambiguous)
		assert.Len(t, ambiguous.AdminIDs, 2, "the error must name every colliding admin so an operator can reconcile them")
	})

	t.Run("an uncontested address resolves normally", func(t *testing.T) {
		admin, err := db.ResolveAdminByEmail(ctx, "SOLO@example.com")
		require.NoError(t, err)
		assert.Equal(t, adminUUID(t, "dddddddd-0000-0000-0000-000000000001"), admin.ID)
	})

	t.Run("a quarantined admin still logs in through its identity", func(t *testing.T) {
		identity, err := db.GetAdminIdentity(ctx, LegacyExternalIDIssuer, "user_dup_loser")
		require.NoError(t, err)
		assert.Equal(t, adminUUID(t, "aaaaaaaa-0000-0000-0000-000000000002"), identity.AdminID,
			"quarantine excludes an admin from the email join, not from authentication")
	})

	t.Run("every seeded external_id became an identity", func(t *testing.T) {
		var count int
		require.NoError(t, conn.GetContext(ctx, &count,
			`SELECT count(*) FROM admin_identities WHERE issuer = $1`, LegacyExternalIDIssuer))
		assert.Equal(t, len(duplicateEmailFixtures(t)), count)
	})

	t.Run("the case-insensitive index is in force", func(t *testing.T) {
		_, err := conn.ExecContext(ctx,
			`INSERT INTO admins (organization_id, email, role)
			 SELECT organization_id, 'SOLO@EXAMPLE.COM', 'owner' FROM admins WHERE email = 'solo@example.com'`)
		require.Error(t, err, "a case-variant duplicate must no longer be insertable")
	})
}

// TestDuplicateEmailReconciliationIsDeterministic runs the same reconciliation
// against databases whose rows were written in different orders. The elected
// keeper must be identical every time: a migration that picks a different
// survivor on a restored backup than on the primary would silently move an
// account between people.
func TestDuplicateEmailReconciliationIsDeterministic(t *testing.T) {
	t.Parallel()

	orders := map[string][]int{
		"reconcile_forward":     {0, 1, 2, 3, 4, 5, 6},
		"reconcile_reversed":    {6, 5, 4, 3, 2, 1, 0},
		"reconcile_interleaved": {1, 3, 5, 0, 2, 4, 6},
	}

	for schema, order := range orders {
		t.Run(schema, func(t *testing.T) {
			t.Parallel()

			_, conn := reconcileDuplicateEmails(t, schema, order)
			ctx := context.Background()

			var keepers []uuid.UUID
			require.NoError(t, conn.SelectContext(ctx, &keepers,
				`SELECT id FROM admins WHERE deleted_at IS NULL AND email_conflict_at IS NULL ORDER BY id`))

			want := make([]uuid.UUID, 0, len(expectedKeepers))
			for _, id := range expectedKeepers {
				want = append(want, adminUUID(t, id))
			}
			assert.Equal(t, want, keepers, "the elected keepers must not depend on physical row order")
		})
	}
}
