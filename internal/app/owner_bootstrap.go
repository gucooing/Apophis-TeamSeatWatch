package app

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"log/slog"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/xft0202/Apophis-TeamSeatWatch/internal/audit"
)

// ensureOwner creates credentials only for an empty installation. The password
// is printed once, after commit, and only its Argon2id hash is persisted.
func ensureOwner(ctx context.Context, dsn string, logger *slog.Logger) error {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return err
	}
	defer pool.Close()
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// Serialize first starts; the table's singleton constraint also protects
	// against a concurrent explicit owner-create command.
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('owner-bootstrap', 0))`); err != nil {
		return err
	}
	var exists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM tsw_owners)`).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return tx.Commit(ctx)
	}
	var raw [24]byte
	if _, err = rand.Read(raw[:]); err != nil {
		return err
	}
	password := base64.RawURLEncoding.EncodeToString(raw[:])
	materials, err := prepareOwnerMaterials(password)
	if err != nil {
		return err
	}
	var ownerID string
	err = tx.QueryRow(ctx, `INSERT INTO tsw_owners (username, password_hash, password_changed_at) VALUES ('owner', $1, now()) RETURNING id`, materials.passwordHash).Scan(&ownerID)
	if err != nil {
		var conflict *pgconn.PgError
		if errors.As(err, &conflict) && conflict.Code == "23505" && (conflict.ConstraintName == "tsw_owners_singleton_uq" || conflict.ConstraintName == "tsw_owners_username_uq") {
			return nil
		}
		return err
	}
	_, err = audit.Write(ctx, tx, audit.Event{
		Type: audit.OwnerCreated, Actor: audit.ActorSystem, OwnerID: ownerID,
		EntityType: "owner", EntityID: ownerID, Outcome: audit.OutcomeSucceeded,
		CorrelationID: "owner-bootstrap", Details: audit.AuthVersionDetails{AuthVersion: 1},
		IdempotencyKey: "owner-created:" + ownerID,
	})
	if err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	logger.Info("owner_initialized", "username", "owner", "password", password)
	return nil
}
