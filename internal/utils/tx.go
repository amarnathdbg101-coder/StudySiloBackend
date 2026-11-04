// Package utils provides utility functions for handling database transactions. It includes the WithTx function, which executes a given function within a database transaction context. The function ensures that the transaction is properly committed or rolled back based on the success or failure of the executed function, as well as handling any panics that may occur during execution.
package utils

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// WithTx executes the given function within a database transaction.
// If the function returns an error or panics, the transaction is rolled back safely
// using an independent context so cancellations do not orphan uncommitted locks.
func WithTx(ctx context.Context, pool *pgxpool.Pool, fn func(tx pgx.Tx) error) (err error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			// Rollback using independent timeout context in case caller ctx is already canceled
			rbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = tx.Rollback(rbCtx)
			panic(p) // re-throw panic after rollback
		} else if err != nil {
			rbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = tx.Rollback(rbCtx)
		} else {
			commitErr := tx.Commit(ctx)
			if commitErr != nil {
				err = fmt.Errorf("failed to commit transaction: %w", commitErr)
			}
		}
	}()

	err = fn(tx)
	return err
}