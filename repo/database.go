package repo

import (
	"errors"
	"fmt"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"path"
	"strings"
	"sync"
)

const (
	dbName = "crawler"
)

// Database is a mutex wrapper around a GORM db.
type Database struct {
	db  *gorm.DB
	mtx sync.Mutex
}

// NewDatabase returns a new database with the given options.
// Sqlite3, Mysql, and Postgress is supported.
func NewDatabase(dataDir string, opts ...Option) (*Database, error) {
	options := Options{
		Host:    "localhost",
		Dialect: "sqlite3",
	}
	if err := options.Apply(opts...); err != nil {
		return nil, err
	}

	dbPath := path.Join(dataDir, dbName)

	switch strings.ToLower(options.Dialect) {
	case "test":
		dbPath = ":memory:"
		options.Dialect = "sqlite3"
	case "mysql":
		dbPath = fmt.Sprintf("%s:%s@(%s:%d)/%s?charset=utf8&parseTime=True", options.User, options.Password, options.Host, options.Port, dbName)
	case "postgress":
		dbPath = fmt.Sprintf("host=%s port=%d user=%s dbname=% password=%s", options.Host, options.Port, options.User, dbName, options.Password)
	case "sqlite3":
		dbPath = dbPath + ".db"
		break
	default:
		return nil, errors.New("unknown database dialect")
	}

	db, err := gorm.Open(options.Dialect, dbPath)
	if err != nil {
		return nil, err
	}

	if err := db.AutoMigrate(&Peer{}, &CIDRecord{}).Error; err != nil {
		return nil, err
	}

	return &Database{db: db}, nil
}

// View is used for read access to the db. Reads are made
// inside and open transaction.
func (d *Database) View(fn func(db *gorm.DB) error) error {
	d.mtx.Lock()
	defer d.mtx.Unlock()

	tx := d.db.Begin()
	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

// Update is used for write access to the db. Updates are made
// inside an open transaction.
func (d *Database) Update(fn func(db *gorm.DB) error) error {
	d.mtx.Lock()
	defer d.mtx.Unlock()

	tx := d.db.Begin()
	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

// Options represents the database options.
type Options struct {
	Host     string
	Port     uint
	Dialect  string
	User     string
	Password string
}

// Apply sets the provided options in the main options struct.
func (o *Options) Apply(opts ...Option) error {
	for i, opt := range opts {
		if err := opt(o); err != nil {
			return fmt.Errorf("option %d failed: %s", i, err)
		}
	}
	return nil
}

// Option represents a db option.
type Option func(*Options) error

// Host option allows you to set the host for mysql or postgress dbs.
func Host(host string) Option {
	return func(o *Options) error {
		o.Host = host
		return nil
	}
}

// Port option sets the port for mysql or postgress dbs.
func Port(port uint) Option {
	return func(o *Options) error {
		o.Port = port
		return nil
	}
}

// Dialect sets the database type...sqlite3, mysql, postress.
func Dialect(dialect string) Option {
	return func(o *Options) error {
		o.Dialect = dialect
		return nil
	}
}

// Password is the password for the mysql or postgress dbs.
func Password(pw string) Option {
	return func(o *Options) error {
		o.Password = pw
		return nil
	}
}

// User is the username for the mysql or postgress dbs.
func User(user string) Option {
	return func(o *Options) error {
		o.User = user
		return nil
	}
}
