package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/KotFed0t/invest_helper_bot/config"
	"github.com/jmoiron/sqlx"
)

// содержит общие методы для sqlx.DB и sqlx.Tx
type Querier interface {
	BindNamed(query string, arg interface{}) (string, []interface{}, error)
    DriverName() string
    Exec(query string, args ...interface{}) (sql.Result, error)
    ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
    Get(dest interface{}, query string, args ...interface{}) error
    GetContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error
    MustExec(query string, args ...interface{}) sql.Result
    MustExecContext(ctx context.Context, query string, args ...interface{}) sql.Result
    NamedExec(query string, arg interface{}) (sql.Result, error)
    NamedExecContext(ctx context.Context, query string, arg interface{}) (sql.Result, error)
    NamedQuery(query string, arg interface{}) (*sqlx.Rows, error)
    Prepare(query string) (*sql.Stmt, error)
    PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
    PrepareNamed(query string) (*sqlx.NamedStmt, error)
    PrepareNamedContext(ctx context.Context, query string) (*sqlx.NamedStmt, error)
    Preparex(query string) (*sqlx.Stmt, error)
    PreparexContext(ctx context.Context, query string) (*sqlx.Stmt, error)
    Query(query string, args ...interface{}) (*sql.Rows, error)
    QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
    QueryRow(query string, args ...interface{}) *sql.Row
    QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
    QueryRowx(query string, args ...interface{}) *sqlx.Row
    QueryRowxContext(ctx context.Context, query string, args ...interface{}) *sqlx.Row
    Queryx(query string, args ...interface{}) (*sqlx.Rows, error)
    QueryxContext(ctx context.Context, query string, args ...interface{}) (*sqlx.Rows, error)
    Rebind(query string) string
    Select(dest interface{}, query string, args ...interface{}) error
    SelectContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error
}

type txKey struct{}

type Postgres struct {
	db  *sqlx.DB
	cfg *config.Config
}

func NewPostgres(cfg *config.Config, db *sqlx.DB) *Postgres {
	return &Postgres{db: db, cfg: cfg}
}

// WithinTransaction runs function within transaction
//
// The transaction commits when function were finished without error
func (p *Postgres) WithinTransaction(ctx context.Context, tFunc func(ctx context.Context) error) error {
	tx, err := p.db.Beginx()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				slog.Error("failed to rollback transaction", slog.String("err", rbErr.Error()))
			}
		}
	}()

	err = tFunc(p.injectTx(ctx, tx))
	if err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// injectTx injects transaction to context
func (p *Postgres) injectTx(ctx context.Context, tx *sqlx.Tx) context.Context {
	return context.WithValue(ctx, txKey{}, tx)
}

// extractTx extracts transaction from context
func (p *Postgres) extractTx(ctx context.Context) *sqlx.Tx {
	if tx, ok := ctx.Value(txKey{}).(*sqlx.Tx); ok {
		return tx
	}
	return nil
}

// txOrDb returns a Querier interface that will use an existing transaction from the context if present,
// otherwise falls back to the provided database connection.
// This allows writing repository methods that work seamlessly with or without transactions.
func (p *Postgres) txOrDb(ctx context.Context) Querier {
	if tx := p.extractTx(ctx); tx != nil {
		return tx
	}
	return p.db
}
