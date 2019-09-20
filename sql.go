package kinoko_web

import (
	"database/sql"
	"errors"
)

type SQL struct {
	Valid                   bool
	MultiDataSources        bool   `inject:"kinoko.sql.multiple-datasource:false"`
	DefaultMultiDataSources string `inject:"kinoko.sql.multiple-default:default"`
	DefaultDataSource       *sql.DB
	DataSources             map[string]*sql.DB
	Configs                 map[interface{}]interface{} `inject:"kinoko.sql.datasource"`
	Proxy                   SQLProxy
}

type DataSource struct {
	DBPools []*sql.DB
	LB      DBLoadBalancer
}

//The sql datasource holder
type SqlPropertiesHolder struct {
	SQL *SQL `inject:""`
}

var sqlPropertiesHolder = SqlPropertiesHolder{}

//
//DataSources configuration sample
//Multiple:
//kinoko:
//  sql:
//    multiple-datasource: true
//    multiple-defualt: db1
// 	  datasource:
//      db1:
//        driverName: mysql
//        url: user:password@tcp(192.168.0.101:3306)/test
//      db2:
//        driverName: mysql
//        url: user:password@tcp(192.168.0.102:3306)/test
//
//Singleton:
//kinoko:
//  sql:
//    datasource:
//      driverName: mysql
//      url: user:password@tcp(192.168.0.101:3306)/test
func (s *SQL) Initialize() error {

	if s.Configs == nil || len(s.Configs) == 0 {
		s.Valid = false
		return nil
	}
	if s.MultiDataSources {
		for k, v := range s.Configs {
			cfg := v.(map[interface{}]interface{})
			url, driver := cfg["url"].(string), cfg["driverName"].(string)
			db, e := sql.Open(driver, url)
			if e != nil {
				return e
			}
			s.DataSources[k.(string)] = db

			if s.DefaultDataSource == nil && k == s.DefaultMultiDataSources {
				s.DefaultDataSource = db
			}
		}
		if s.DefaultDataSource == nil {
			return errors.New("you must provide a default datasource for multiple datasource in config")
		}
	} else {
		db, e := sql.Open(s.Configs["url"].(string), s.Configs["driverName"].(string))
		if e != nil {
			return e
		}
		s.DefaultDataSource = db
		s.DataSources["default"] = db
	}
	s.Valid = true
	return nil
}
