package data

import (
	"context"
	stdsql "database/sql"
	"fmt"

	"github.com/liu-jiangyuan/kratos-template/core/conf"
	"github.com/liu-jiangyuan/kratos-template/ent"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/go-kratos/kratos/v3/log"
	"github.com/google/wire"
	_ "github.com/lib/pq"
)

// ProviderSet is core data providers.
var ProviderSet = wire.NewSet(NewData)

// txKey identifies an active transaction stored in context.Context.
type txKey struct{}

// Data holds the long-lived storage clients shared by module model layers.
type Data struct {
	db *ent.Client
}

// NewDataWithClient constructs a Data wrapper around an existing ent.Client
// (primarily used by repository unit tests with an in-memory SQLite client).
func NewDataWithClient(client *ent.Client) *Data {
	return &Data{db: client}
}

// NewData opens the database client and returns it with a cleanup function.
func NewData(c *conf.Data) (*Data, func(), error) {
	dc := c.GetDatabase()
	driverName := dc.GetDriver()
	if driverName == "" {
		driverName = dialect.Postgres
	}
	sqlDialect := driverName
	if driverName == "pgx" {
		sqlDialect = dialect.Postgres
	}
	sqlDB, err := stdsql.Open(driverName, dc.GetSource())
	if err != nil {
		return nil, nil, err
	}
	db := ent.NewClient(ent.Driver(entsql.OpenDB(sqlDialect, sqlDB)))
	if dc.GetDebug() {
		db = db.Debug()
	}
	// Auto migration is a convenience for local development. In production,
	// apply schema changes as a separate reviewed step instead.
	if dc.GetAutoMigrate() {
		if err := db.Schema.Create(context.Background()); err != nil {
			_ = db.Close()
			return nil, nil, err
		}
	}
	cleanup := func() {
		log.Info("closing the data resources")
		if err := db.Close(); err != nil {
			log.Error("failed closing the database", "err", err)
		}
	}
	return &Data{db: db}, cleanup, nil
}

// DB returns the active transactional ent.Client from ctx if within InTx,
// otherwise the base ent.Client.
func (d *Data) DB(ctx context.Context) *ent.Client {
	if tx, ok := ctx.Value(txKey{}).(*ent.Tx); ok && tx != nil {
		return tx.Client()
	}
	return d.db
}

// InTx executes fn inside a database transaction. Nested calls reuse the outer
// transaction from ctx.
func (d *Data) InTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(txKey{}).(*ent.Tx); ok {
		return fn(ctx)
	}
	tx, err := d.db.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if v := recover(); v != nil {
			_ = tx.Rollback()
			panic(v)
		}
	}()
	if err := fn(context.WithValue(ctx, txKey{}, tx)); err != nil {
		if rerr := tx.Rollback(); rerr != nil {
			return fmt.Errorf("%w: rolling back transaction: %v", err, rerr)
		}
		return err
	}
	return tx.Commit()
}
