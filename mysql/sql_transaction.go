package mysql

import (
	"context"
	"database/sql"
	"errors"
	"github.com/samber/lo"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"runtime"
)

type (
	InTransactionHandler    func(ctx context.Context, db boil.ContextExecutor) error
	TransactionScopeWrapper func(executeInTx InTransactionHandler) error
	TransactionScopeBuilder func(ctx context.Context, txIsolationLevel sql.IsolationLevel) (TransactionScopeWrapper, error)

	contextWithTx struct {
		context.Context
		tx       *sql.Tx
		isClosed bool
	}
)

var (
	ErrTxAlreadyClosed = errors.New("transaction is already closed")
	TxModule           = fx.Module(
		"mysqlTx",
		fx.Provide(InitializeModule),
	)
)

func InitializeModule(db *sql.DB, log *zap.SugaredLogger) TransactionScopeBuilder {

	return func(ctx context.Context, isolationLevel sql.IsolationLevel) (TransactionScopeWrapper, error) {
		var targetCtx contextWithTx

		txCtx, isInParentScope := ctx.(contextWithTx)

		if isInParentScope {
			log.Debugf("creating child transaction scope")
			targetCtx = txCtx
		} else {
			log.Debugf("creating parent transaction scope")
			tx, err := db.BeginTx(ctx, &sql.TxOptions{
				Isolation: isolationLevel,
				ReadOnly:  false,
			})

			if err != nil {
				log.Errorf("could not initialize db transaction: %v", err)
				return nil, err
			}

			targetCtx = contextWithTx{
				Context:  ctx,
				tx:       tx,
				isClosed: false,
			}

			runtime.SetFinalizer(&targetCtx, buildTxFinalizer(log))
		}

		return func(executeInTx InTransactionHandler) error {
			if targetCtx.isClosed {
				log.Warnf("trying to use already closed transaction")
				return ErrTxAlreadyClosed
			}

			err := executeInTx(targetCtx, targetCtx.tx)
			if !isInParentScope {
				targetCtx.isClosed = true
				log.Debugf("about to complete tx - result %s", lo.Ternary(err == nil, "COMMIT", "ROLLBACK"))

				innerErr := lo.IfF(err == nil, targetCtx.tx.Commit).ElseF(targetCtx.tx.Rollback)
				return lo.Ternary(innerErr != nil, innerErr, err)
			}
			log.Debugf("child tx scope completed - result %s", lo.Ternary(err == nil, "COMMIT", "ROLLBACK"))
			return err
		}, nil
	}
}

func buildTxFinalizer(log *zap.SugaredLogger) func(ctx *contextWithTx) {
	return func(ctx *contextWithTx) {
		if !ctx.isClosed {
			err := ctx.tx.Rollback()
			log.Errorf("transaction is not closed but got out of scope - applying rollback: %v", err)
		}
	}
}
