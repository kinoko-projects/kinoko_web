package kinoko_web

import (
	"context"
	"database/sql"
	"time"
)

type SQLSession struct {
	Transactional bool
	ctx           context.Context
	tx            *sql.Tx
	db            *sql.DB
	exec          executor
	err           error
}

type SQLExecutor interface {
	ExecuteN(sql string, parameters ...interface{}) (int64, error)
	ExecuteI(sql string, parameters ...interface{}) (int64, error)
	Query(sql string, parameters ...interface{}) (*sql.Rows, error)
}

//TODO proxy (paging) & mapper
//type SQLExecutorProxy interface {
//	executeNProxy(executor SQLExecutor)
//	queryProxy(executor SQLExecutor)
//}

//The original executor of db & tx
type executor interface {
	ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
	QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error)
}

func (s *SQLSession) BeginTx(options ...*sql.TxOptions) {
	if s.tx != nil {
		panic("transaction is not committed yet")
	}
	if len(options) == 0 {
		s.tx, s.err = s.db.BeginTx(s.ctx, nil)
	} else {
		s.tx, s.err = s.db.BeginTx(s.ctx, options[0])
	}
	if s.err != nil {
		panic(s.err)
	}
	s.Transactional = true
	s.exec = s.tx
}

func (s *SQLSession) Rollback() {
	if s.Transactional {
		e := s.tx.Rollback()
		if e != nil {
			panic(e)
		}
		s.tx = nil
		s.exec = nil
		s.Transactional = false
	}
}

func (s *SQLSession) Commit() {
	if s.Transactional {
		e := s.tx.Commit()
		if e != nil {
			panic(e)
		}
		s.tx = nil
		s.exec = nil
		s.Transactional = false
	}
}

//execute and return id of new row
func (s *SQLSession) ExecuteI(query string, args ...interface{}) (int64, error) {
	result, e := s.exec.ExecContext(s.ctx, query, args)
	if e != nil {
		return 0, e
	}
	return result.LastInsertId()
}

//execute and return number of rows affected
func (s *SQLSession) ExecuteN(query string, args ...interface{}) (int64, error) {
	result, e := s.exec.ExecContext(s.ctx, query, args)
	if e != nil {
		return 0, e
	}
	return result.RowsAffected()
}

func (s *SQLSession) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return s.exec.QueryContext(s.ctx, query, args)
}

func (s *SQLSession) SwitchDataSource(source string) {
	if s.Transactional {
		panic("Switch with uncommitted transaction")
	}
	db := sqlPropertiesHolder.SQL.DataSources[source]
	if db == nil {
		panic("No such datasource - " + source)
	}
	s.db = db
}

func newSQLSession(timeout time.Duration, datasource *sql.DB) *SQLSession {
	if timeout > 0 {
		withTimeout, _ := context.WithTimeout(context.Background(), timeout)
		return &SQLSession{ctx: withTimeout, db: datasource}
	} else {
		return &SQLSession{ctx: context.Background(), db: datasource}
	}
}
