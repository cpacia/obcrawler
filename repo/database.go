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

type Database struct {
	db  *gorm.DB
	mtx sync.Mutex
}

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

type Options struct {
	Host     string
	Port     uint
	Dialect  string
	User     string
	Password string
}

func (o *Options) Apply(opts ...Option) error {
	for i, opt := range opts {
		if err := opt(o); err != nil {
			return fmt.Errorf("option %d failed: %s", i, err)
		}
	}
	return nil
}

type Option func(*Options) error

func Host(host string) Option {
	return func(o *Options) error {
		o.Host = host
		return nil
	}
}

func Port(port uint) Option {
	return func(o *Options) error {
		o.Port = port
		return nil
	}
}

func Dialect(dialect string) Option {
	return func(o *Options) error {
		o.Dialect = dialect
		return nil
	}
}

func Password(pw string) Option {
	return func(o *Options) error {
		o.Password = pw
		return nil
	}
}

func User(user string) Option {
	return func(o *Options) error {
		o.User = user
		return nil
	}
}
