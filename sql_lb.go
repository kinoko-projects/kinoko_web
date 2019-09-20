package kinoko_web

import (
	"database/sql"
	"time"
)

type DBLoadBalancer interface {
	Init(source *DataSource, config map[interface{}]interface{})
	Select(source *DataSource) *sql.DB
	Error(source *DataSource, err error)
}

//Balancer without op, just return index 0,(no cluster)
type LBNoOp struct {
}

func (*LBNoOp) Init(source *DataSource, config map[interface{}]interface{}) {
	//noop
}

func (*LBNoOp) Error(source *DataSource, err error) {
	//noop
}

func (*LBNoOp) Select(source *DataSource) *sql.DB {
	return source.DBPools[0]
}

type LBRoundRobin struct {
	pos   uint
	after []time.Time
}
