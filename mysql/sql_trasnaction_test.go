package mysql

import (
	"context"
	"database/sql"
	"errors"
	"github.com/armory-io/go-commons/integration_utils"
	"github.com/armory-io/go-commons/logging"
	"github.com/go-sql-driver/mysql"
	"github.com/sirupsen/logrus"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"go.uber.org/zap/zapcore"
	"gotest.tools/assert"
	"testing"
	"time"
)

func TestSqlTransaction(t *testing.T) {
	l, err := logging.StdArmoryDevLogger(zapcore.DebugLevel)
	if err != nil {
		t.Fatal(err)
	}
	logger := l.Sugar()

	conn := integration_utils.CreateIntegrationDatabase(t)

	mysqlDb, err := sql.Open("mysql", conn)
	if err != nil {
		t.Fatal(err)
	}
	err = createInitialData(mysqlDb)
	if err != nil {
		t.Fatal(err)
	}

	txScopeBuilder := InitializeModule(mysqlDb, logger)

	defer mysqlDb.Close()

	cases := []struct {
		name     string
		testCase func(c *testing.T)
	}{
		{
			name: "returning from tx without error commits transaction",
			testCase: func(c *testing.T) {
				txScopeWrapper, err := txScopeBuilder(context.TODO(), sql.LevelReadCommitted)
				assert.NilError(t, err)
				err = txScopeWrapper(func(ctx context.Context, db boil.ContextExecutor) error {
					_, err := db.Exec("insert into cars(idx, name, price) values (4, 'mini', 99999)")
					return err
				})

				assert.NilError(t, err)
				row := mysqlDb.QueryRow("select idx from cars where name = 'mini'")
				assert.NilError(t, row.Err())
				var id int
				_ = row.Scan(&id)
				assert.Equal(t, 4, id)
			},
		},
		{
			name: "fail within TX scope will rollback the changes",
			testCase: func(c *testing.T) {
				txScopeWrapper, err := txScopeBuilder(context.TODO(), sql.LevelReadCommitted)
				assert.NilError(t, err)
				err = txScopeWrapper(func(ctx context.Context, db boil.ContextExecutor) error {
					_, err := db.Exec("insert into cars(idx, name, price) values (5, 'bentley', 99999)")
					_, err = db.Exec("insert into cars(idx, name, price) values (5, 'bugatti', 99999)")
					return err
				})
				_, isMySqlError := err.(*mysql.MySQLError)
				assert.Equal(t, true, isMySqlError)
				row := mysqlDb.QueryRow("select count(idx) from cars where idx = 5")
				assert.NilError(t, row.Err())
				var cnt int
				_ = row.Scan(&cnt)
				assert.Equal(t, 0, cnt)
			},
		},
		{
			name: "will fail if trying to use already closed tx",
			testCase: func(c *testing.T) {
				txScopeWrapper, err := txScopeBuilder(context.TODO(), sql.LevelReadCommitted)
				assert.NilError(t, err)
				err = txScopeWrapper(func(ctx context.Context, db boil.ContextExecutor) error {
					_, err := db.Exec("insert into cars(idx, name, price) values (6, 'skoda', 1000)")
					return err
				})
				assert.NilError(t, err)
				err = txScopeWrapper(func(ctx context.Context, db boil.ContextExecutor) error {
					_, err := db.Exec("insert into cars(idx, name, price) values (7, 'saab', 5000)")
					return err
				})
				assert.Equal(t, true, errors.Is(err, ErrTxAlreadyClosed))
			},
		},
		{
			name: "transaction scopes are separated",
			testCase: func(c *testing.T) {
				sync := make(chan bool)
				done := make(chan error)
				txScopeWrapperWrite, err := txScopeBuilder(context.TODO(), sql.LevelReadCommitted)
				assert.NilError(t, err)
				txScopeWrapperRead, err := txScopeBuilder(context.TODO(), sql.LevelReadCommitted)
				assert.NilError(t, err)

				go func() {
					err = txScopeWrapperRead(func(ctx context.Context, db boil.ContextExecutor) error {
						_ = <-sync
						row := db.QueryRow("select count(idx) from cars where name = 'seat'")
						assert.NilError(t, row.Err())
						var cnt int
						_ = row.Scan(&cnt)
						assert.Equal(t, 0, cnt)
						sync <- true
						logrus.Warn("read scope complete!")
						return err
					})
					done <- err
				}()

				time.Sleep(time.Second)

				go func() {
					err = txScopeWrapperWrite(func(ctx context.Context, db boil.ContextExecutor) error {
						_, err := db.Exec("insert into cars(idx, name, price) values (8, 'seat', 1000)")
						sync <- true
						time.Sleep(time.Second)
						_ = <-sync
						logrus.Warn("write scope complete!")
						return err
					})
					done <- err
				}()

				err1, err2 := <-done, <-done

				assert.NilError(t, err1)
				assert.NilError(t, err2)
			},
		},
		{
			name: "wrapped scope - outer scope commits",
			testCase: func(c *testing.T) {
				txScopeWrapper, err := txScopeBuilder(context.TODO(), sql.LevelReadCommitted)
				assert.NilError(t, err)
				err = txScopeWrapper(func(ctx context.Context, db boil.ContextExecutor) error {
					innerScope, err := txScopeBuilder(ctx, sql.LevelReadCommitted)
					if err != nil {
						t.Fatal(err)
					}

					err = innerScope(func(innerCtx context.Context, innerDb boil.ContextExecutor) error {
						_, err = innerDb.Exec("insert into cars(idx, name, price) values (10, 'jaguar', 2500)")
						return err
					})

					if err != nil {
						t.Fatal(err)
					}
					_, err = db.Exec("insert into cars(idx, name, price) values (9, 'jeep', 1500)")
					return err
				})
				assert.NilError(t, err)
				row := mysqlDb.QueryRow("select count(*) from cars where name like 'j%'")
				assert.NilError(t, row.Err())
				var cnt int
				_ = row.Scan(&cnt)
				assert.Equal(t, 2, cnt)
			},
		},
		{
			name: "wrapped scope - outer scope rollbacks",
			testCase: func(c *testing.T) {
				txScopeWrapper, err := txScopeBuilder(context.TODO(), sql.LevelReadCommitted)
				assert.NilError(t, err)
				err = txScopeWrapper(func(ctx context.Context, db boil.ContextExecutor) error {
					innerScope, err := txScopeBuilder(ctx, sql.LevelReadCommitted)
					if err != nil {
						t.Fatal(err)
					}

					err = innerScope(func(innerCtx context.Context, innerDb boil.ContextExecutor) error {
						_, err = innerDb.Exec("insert into cars(idx, name, price) values (11, 'toyota', 2500)")
						return err
					})

					if err != nil {
						t.Fatal(err)
					}
					_, err = db.Exec("insert into cars(idx, name, price) values (11, 'talbot', 500)")
					return err
				})
				_, isMySqlError := err.(*mysql.MySQLError)
				assert.Equal(t, true, isMySqlError)

				row := mysqlDb.QueryRow("select count(*) from cars where name like 't%'")
				assert.NilError(t, row.Err())
				var cnt int
				_ = row.Scan(&cnt)
				assert.Equal(t, 0, cnt)
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t2 *testing.T) {
			c.testCase(t2)
		})
	}
}

func createInitialData(db *sql.DB) error {
	sts := []string{"INSERT INTO cars(idx, name, price) VALUES(1, 'Audi', 10000)",
		"INSERT INTO cars(idx, name, price) VALUES(2, 'Mercedes', 11000)",
		"INSERT INTO cars(idx, name, price) VALUES(3, 'BMW', 12000)",
	}
	_, err := db.Exec("CREATE TABLE cars(idx integer not null, name varchar(64), price integer, primary key (idx))")
	if err != nil {
		return err
	}
	for _, row := range sts {
		_, err = db.Exec(row)
		if err != nil {
			return err
		}
	}
	return nil
}
