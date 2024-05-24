package mysql

import (
	"database/sql"
	"errors"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"k8s.io/klog/v2"
)

type DBConfig struct {
	User     string
	Password string
	Host     string
	Port     string
	Database string
}

type DB struct {
	*sqlx.DB
}

func NewDorisSqlDB(cfg DBConfig) (*DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Database)
	db, err := sqlx.Open("mysql", dsn)
	if err != nil {
		klog.Errorf("failed open doris sql client connection, err: %s \n", err)
		return nil, err
	}

	if err = db.Ping(); err != nil {
		klog.Errorf("failed ping doris sql client connection, err: %s\n", err.Error())
		return nil, err
	}
	return &DB{db}, nil
}

func (db *DB) Exec(query string, args ...interface{}) (sql.Result, error) {
	return db.DB.Exec(query, args...)
}

func (db *DB) Select(dest interface{}, query string, args ...interface{}) error {
	return db.DB.Select(dest, query, args...)
}

func (db *DB) ShowFrontends() ([]Frontend, error) {
	var fes []Frontend
	err := db.Select(&fes, "show frontends")
	return fes, err
}

func (db *DB) ShowBackends() ([]Backend, error) {
	var bes []Backend
	err := db.Select(&bes, "show backends")
	return bes, err
}

func (db *DB) DecommissionBE(nodes []Backend) error {
	if len(nodes) == 0 {
		klog.Infoln("mysql DecommissionBE BE node is empty")
		return nil
	}
	nodesString := fmt.Sprintf(`"%s:%d"`, nodes[0].Host, nodes[0].HeartbeatPort)
	for _, node := range nodes[1:] {
		nodesString = nodesString + fmt.Sprintf(`,"%s:%d"`, node.Host, node.HeartbeatPort)
	}

	alter := fmt.Sprintf("ALTER SYSTEM DECOMMISSION BACKEND %s;", nodesString)
	_, err := db.Exec(alter)
	return err
}

func (db *DB) CheckDecommissionBE(nodes []Backend) (isFinished bool, err error) {
	backends, err := db.ShowBackends()
	info := NewDecommissionInfo(backends, nodes)
	return info.IsFinished(), err
}

func (db *DB) DropObserver(nodes []Frontend) error {
	if len(nodes) == 0 {
		klog.Infoln("mysql DropObserver observer node is empty")
		return nil
	}
	var alter string
	for _, node := range nodes {
		alter = alter + fmt.Sprintf(`ALTER SYSTEM DROP OBSERVER "%s:%d";`, node.Host, node.EditLogPort)
	}
	_, err := db.Exec(alter)
	return err
}

func (db *DB) GetMaster() (*Frontend, error) {
	frontends, err := db.ShowFrontends()
	if err != nil {
		klog.Errorf("GetMaster show frontends failed, err: %s\n", err.Error())
		return nil, err
	}
	for _, fe := range frontends {
		if fe.IsMaster {
			return &fe, nil
		}
	}
	errMessage := fmt.Sprintf("GetMaster note not find fe master, all of fe nodes info as such: %+v", frontends)
	return nil, errors.New(errMessage)
}