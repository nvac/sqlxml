package sqlxml

import (
	"context"
	"database/sql"
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"time"

	"github.com/jmoiron/sqlx"
)

//region Error

var ErrNoScript = errors.New("sqlxml: no script in config file")

//endregion

//region Options

type Options struct {
	DatabaseFile     string
	ScriptsGlobFiles string
	Env              string
	DsnDecryptFunc   func(dsn string) string
}

//endregion

//region XML
type databasesXml struct {
	XMLName   xml.Name      `xml:"databases"`
	Databases []databaseXml `xml:"database"`
}

type databaseXml struct {
	XMLName                xml.Name `xml:"database"`
	Name                   string   `xml:"name,attr"`
	Driver                 string   `xml:"driver,attr"`
	Dsn                    string   `xml:"dsn,attr"`
	Env                    string   `xml:"env,attr"`
	MaxIdleConns           *int     `xml:"maxIdleConns,attr"`
	MaxOpenConns           *int     `xml:"maxOpenConns,attr"`
	ConnMaxLifetimeSeconds *int     `xml:"connMaxLifetimeSeconds,attr"`
	ConnMaxIdleTimeSeconds *int     `xml:"connMaxIdleTimeSeconds,attr"`
}

type scriptsXml struct {
	XMLName xml.Name    `xml:"scripts"`
	Scripts []scriptXml `xml:"script"`
}

type scriptXml struct {
	XMLName xml.Name `xml:"script"`
	Name    string   `xml:"name,attr"`
	Content string   `xml:",chardata"`
}

//endregion

//region Client

type Client struct {
	dbMap     map[string]*sqlx.DB
	scriptMap map[string]string
	err       error
}

func (c *Client) Error() error {
	return c.err
}

func (c *Client) Database(dbName string) *Database {
	d := &Database{
		client: c,
	}

	if db, ok := c.dbMap[dbName]; ok {
		d.db = db
	} else {
		d.err = fmt.Errorf("the database name(%s) is not found", dbName)
	}

	return d
}

func NewClient(opt *Options) *Client {
	c := &Client{}
	if opt.DatabaseFile == "" {
		c.err = errors.New("DatabaseFile is required")
		return c
	}

	if opt.ScriptsGlobFiles == "" {
		c.err = errors.New("ScriptsGlobFiles is required")
		return c
	}

	if dbMap, err := loadDatabasesFile(opt); err != nil {
		c.err = err
		return c
	} else {
		c.dbMap = dbMap
	}

	if scriptMap, err := loadScriptsGlobFiles(opt); err != nil {
		c.err = err
		return c
	} else {
		c.scriptMap = scriptMap
	}

	return c
}

//endregion

//region Database

type Database struct {
	client *Client
	db     *sqlx.DB
	err    error
}

func (d *Database) Error() error {
	return d.err
}

func (d *Database) QueryRow(ctx context.Context, scriptName string, arg any, result any) error {
	nStmt, err := GetNStmt(ctx, d, scriptName)
	if err != nil {
		return err
	}
	defer func() { _ = nStmt.Close() }()

	return nStmt.GetContext(ctx, result, arg)
}

func (d *Database) QueryRowByMap(ctx context.Context, scriptName string, arg map[string]any, result any) error {
	nStmt, err := GetNStmt(ctx, d, scriptName)
	if err != nil {
		return err
	}
	defer func() { _ = nStmt.Close() }()

	return nStmt.GetContext(ctx, result, arg)
}

func (d *Database) QueryRows(ctx context.Context, scriptName string, arg any, result any) error {
	nStmt, err := GetNStmt(ctx, d, scriptName)
	if err != nil {
		return err
	}
	defer func() { _ = nStmt.Close() }()

	return nStmt.SelectContext(ctx, result, arg)
}

func (d *Database) QueryRowsByMap(ctx context.Context, scriptName string, arg map[string]any, result any) error {
	return d.QueryRows(ctx, scriptName, arg, result)
}

func (d *Database) Exec(ctx context.Context, scriptName string, arg any) (sql.Result, error) {
	nStmt, err := GetNStmt(ctx, d, scriptName)
	if err != nil {
		return nil, err
	}
	defer func() { _ = nStmt.Close() }()

	return nStmt.ExecContext(ctx, arg)
}

func GetNStmt(ctx context.Context, d *Database, scriptName string) (*sqlx.NamedStmt, error) {
	script, ok := d.client.scriptMap[scriptName]
	if !ok {
		return nil, ErrNoScript
	}

	return d.db.PrepareNamedContext(ctx, script)
}

//endregion

//region Util

func loadDatabasesFile(opt *Options) (map[string]*sqlx.DB, error) {
	file, err := ioutil.ReadFile(opt.DatabaseFile)
	if err != nil {
		return nil, err
	}

	data := new(databasesXml)
	err = xml.Unmarshal(file, data)
	if err != nil {
		return nil, err
	}

	if data == nil || len(data.Databases) == 0 {
		return nil, errors.New("no available database")
	}

	databaseMap := make(map[string]*sqlx.DB)
	for _, dbXml := range data.Databases {
		if opt.Env != "" && opt.Env != dbXml.Env {
			continue
		}

		dsn := dbXml.Dsn
		if opt.DsnDecryptFunc != nil {
			dsn = opt.DsnDecryptFunc(dbXml.Dsn)
		}

		db, err := sqlx.Open(dbXml.Driver, dsn)
		if err != nil {
			return nil, err
		}

		if dbXml.MaxOpenConns != nil {
			db.SetMaxOpenConns(*dbXml.MaxOpenConns)
		}

		if dbXml.MaxIdleConns != nil {
			db.SetMaxIdleConns(*dbXml.MaxIdleConns)
		}

		if dbXml.ConnMaxLifetimeSeconds != nil {
			seconds := time.Duration(*dbXml.ConnMaxLifetimeSeconds)
			db.SetConnMaxLifetime(seconds * time.Second)
		}

		if dbXml.ConnMaxLifetimeSeconds != nil {
			seconds := time.Duration(*dbXml.ConnMaxLifetimeSeconds)
			db.SetConnMaxIdleTime(seconds * time.Second)
		}

		databaseMap[dbXml.Name] = db
	}

	return databaseMap, nil
}

func loadScriptsGlobFiles(opt *Options) (map[string]string, error) {
	var scriptMap map[string]string

	scriptFilePathList, _ := filepath.Glob(opt.ScriptsGlobFiles)
	for _, scriptFilePath := range scriptFilePathList {
		fileContent, err := ioutil.ReadFile(scriptFilePath)
		if err != nil {
			return nil, err
		}

		data := new(scriptsXml)
		err = xml.Unmarshal(fileContent, data)
		if err != nil {
			return nil, err
		}

		for _, script := range data.Scripts {
			if _, ok := scriptMap[script.Name]; ok {
				return nil, fmt.Errorf("the script name(%s) is duplicate", script.Name)
			} else {
				scriptMap[script.Name] = script.Content
			}
		}
	}

	return scriptMap, nil
}

//endregion
