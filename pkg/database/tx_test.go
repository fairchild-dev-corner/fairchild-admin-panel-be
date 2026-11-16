package database

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"testing"
)

type txState struct {
	begins, commits, rollbacks int
	beginErr, commitErr        error
}
type txConnector struct{ state *txState }

func (c txConnector) Connect(context.Context) (driver.Conn, error) {
	return &testConn{state: c.state}, nil
}
func (c txConnector) Driver() driver.Driver { return testDriver{} }

type testDriver struct{}

func (testDriver) Open(string) (driver.Conn, error) { return nil, errors.New("use connector") }

type testConn struct{ state *txState }

func (c *testConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("unexpected prepare") }
func (c *testConn) Close() error                        { return nil }
func (c *testConn) Begin() (driver.Tx, error) {
	c.state.begins++
	if c.state.beginErr != nil {
		return nil, c.state.beginErr
	}
	return &testTx{state: c.state}, nil
}

type testTx struct{ state *txState }

func (t *testTx) Commit() error   { t.state.commits++; return t.state.commitErr }
func (t *testTx) Rollback() error { t.state.rollbacks++; return nil }
func TestTransactionCommitRollbackAndErrors(t *testing.T) {
	failure := errors.New("failure")
	for _, tc := range []struct {
		name                         string
		beginErr, bodyErr, commitErr error
		commits, rollbacks           int
	}{
		{"commit", nil, nil, nil, 1, 0},
		{"rollback", nil, failure, nil, 0, 1},
		{"begin fails", failure, nil, nil, 0, 0},
		{"commit fails", nil, nil, failure, 1, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := &txState{beginErr: tc.beginErr, commitErr: tc.commitErr}
			db := sql.OpenDB(txConnector{state: state})
			defer db.Close()
			called := false
			err := WithTx(context.Background(), db, func(*sql.Tx) error { called = true; return tc.bodyErr })
			if (err != nil) != (tc.beginErr != nil || tc.bodyErr != nil || tc.commitErr != nil) {
				t.Fatalf("unexpected error %v", err)
			}
			if err != nil && !errors.Is(err, failure) {
				t.Fatal("error not preserved", err)
			}
			if called != (tc.beginErr == nil) || state.commits != tc.commits || state.rollbacks != tc.rollbacks {
				t.Fatalf("called=%v state=%+v", called, state)
			}
		})
	}
}
