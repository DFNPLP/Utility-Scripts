package persistence

import (
	"database/sql"
	"fmt"
	"regexp"
	"sync"

	_ "modernc.org/sqlite" // enables sqlite to be used as the DB driver
)

const CURRENT_DATA_SCHEMA_VERSION uint32 = 1
const DEFAULT_MAX_FILE_PATH_LENGTH_IN_DB uint32 = 4000 //todo, check into this...I have to set the size...might want to set it as a const up above

type FimulcrumDbAccessLayer struct {
	//todo, make sure these are not empty or otherwise invalid
	sourceDbName string
	targetDbName string
	tagsDbName   string
	// plannedActionsDbName string

	db *sql.DB
	//sqlite doesn't do well with concurrent accesses via the same connection, so we share a lock
	dataMutex *sync.Mutex

	hashSize uint32
}

func getSqliteTableNameValidationRegex() (string, *regexp.Regexp, error) {
	regexString := "[a-zA-Z0-9_]+"
	regex, err := regexp.Compile("[a-zA-Z0-9_]+")
	return regexString, regex, err
}

// todo, need to store the hash size in some way that it can be retrived along with the schema version
func NewFimulcrumDbAccessLayerWithDefaults(pathToDb string, hashSize uint32) (*FimulcrumDbAccessLayer, error) {
	//todo, check if this exists, if it does we need to check versions and try to load it
	db, err := sql.Open("sqlite", pathToDb)
	if err != nil {
		return nil, err
	}

	return NewFimulcrumDbAccessLayer(
		db,
		"source_data",
		"target_data",
		"tags_catalog",
		DEFAULT_MAX_FILE_PATH_LENGTH_IN_DB,
		hashSize,
	)
}

// BE CAREFUL, this function forces some basic validation of table names to prevent an injection which...
// honestly isn't likely useful. But, being safe and making validation of user input a habit is a good thing.
func NewFimulcrumDbAccessLayer(
	db *sql.DB,
	sourceDbName string,
	targetDbName string,
	tagsDbName string,
	maxFilepathLength uint32,
	hashSize uint32,
) (*FimulcrumDbAccessLayer, error) {
	fdal := &FimulcrumDbAccessLayer{
		sourceDbName: sourceDbName,
		targetDbName: targetDbName,
		tagsDbName:   tagsDbName,

		db:        db,
		dataMutex: &sync.Mutex{},

		hashSize: hashSize,
	}

	regexString, tableNameValidationRegex, err := getSqliteTableNameValidationRegex()
	if err != nil {
		return nil, err
	}
	for _, item := range []string{fdal.sourceDbName, fdal.targetDbName} {
		//validate the table names
		matched := tableNameValidationRegex.Match([]byte(item))
		if !matched {
			return nil, fmt.Errorf("%s table name must match the regex: %s", item, regexString)
		}
	}
	//todo, need some way to guarantee that it was actually fimulcrum that created this DB, too....I think there might be anoother pragma?
	_, err = fdal.db.Exec(fmt.Sprintf("PRAGMA user_version=%d", CURRENT_DATA_SCHEMA_VERSION))
	if err != nil {
		return nil, err
	}

	for _, item := range []string{fdal.sourceDbName, fdal.targetDbName} {
		//todo does path itself need to have a constraint that it's unique? I'm thinking yes?
		_, err = fdal.db.Exec(
			fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s
				(
					id UNSIGNED BIG INT PRIMARY KEY,
					hash BYTES(%d) NOT NULL,
					path NVARCHAR(%d) NOT NULL,
					name MEDIUMTEXT NOT NULL,
					newName MEDIUMTEXT NOT NULL,
					UNIQUE (hash, path)
				);`,
				item,
				maxFilepathLength,
				hashSize,
			),
		)
		if err != nil {
			return nil, err
		}
	}
	_, err = fdal.db.Exec(
		//todo are 64 and 256 the sizes I want here?
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s
			(
				id UNSIGNED BIG INT PRIMARY KEY,
				name nvarchar(64) NOT NULL,
				comment NVARCHAR(256),
				actionId UNSIGNED BIG INT PRIMARY KEY,
				UNIQUE (name)
			);`,
			fdal.tagsDbName,
		),
	)
	if err != nil {
		return nil, err
	}

	return fdal, nil
}

func (fdal *FimulcrumDbAccessLayer) PersistNewSourceHashRecord(data *HashData) error {
	return fdal.persistNewHashRecord(fdal.sourceDbName, data)
}

func (fdal *FimulcrumDbAccessLayer) PersistNewTargetHashRecord(data *HashData) error {
	return fdal.persistNewHashRecord(fdal.targetDbName, data)
}

func (fdal *FimulcrumDbAccessLayer) persistNewHashRecord(tableName string, data *HashData) error {
	fdal.dataMutex.Lock()
	defer fdal.dataMutex.Unlock()
	_, err := fdal.db.Exec(
		fmt.Sprintf("INSERT INTO %s (hash, path, name, newName) VALUES (?, ?, ?, ?)", tableName),
		data.Hash,
		data.Path,
		data.Name,
		data.NewName,
	)
	return err
}

func (fdal *FimulcrumDbAccessLayer) GetHashRecordsByHash(tableName string, hash []byte) ([]*HashData, error) {
	return fdal.getHashRecords(fdal.targetDbName, 0, nil)
}

func (fdal *FimulcrumDbAccessLayer) GetSourceHashRecords(limit uint) ([]*HashData, error) {
	return fdal.getHashRecords(fdal.sourceDbName, limit, nil)
}

func (fdal *FimulcrumDbAccessLayer) GetTargetHashRecords(limit uint) ([]*HashData, error) {
	return fdal.getHashRecords(fdal.targetDbName, limit, nil)
}

// todo, need to add a skip parameter
func (fdal *FimulcrumDbAccessLayer) getHashRecords(tableName string, limit uint, hash []byte) ([]*HashData, error) {
	fdal.dataMutex.Lock()
	defer fdal.dataMutex.Unlock()
	resultData := make([]*HashData, 0)
	var resultSet *sql.Rows
	var err error

	//will need to adjust this if they get slightly more complex....
	if limit > 0 {
		if len(hash) == 0 {
			resultSet, err = fdal.db.Query(
				fmt.Sprintf("SELECT id, hash, path, name, newName FROM %s LIMIT ?;", tableName),
				limit,
			)
		} else {
			resultSet, err = fdal.db.Query(
				fmt.Sprintf("SELECT id, hash, path, name, newName FROM %s WHERE hash = ? LIMIT ?;", tableName),
				hash,
				limit,
			)
		}
	} else {
		if len(hash) == 0 {
			resultSet, err = fdal.db.Query(
				fmt.Sprintf("SELECT id, hash, path, name, newName FROM %s;", tableName),
				limit,
			)
		} else {
			resultSet, err = fdal.db.Query(
				fmt.Sprintf("SELECT id, hash, path, name, newName FROM %s WHERE hash = ?;", tableName),
				hash,
				limit,
			)
		}
	}

	if err != nil {
		return nil, err
	}
	defer resultSet.Close()

	for resultSet.Next() {
		rowData := &HashData{
			Hash: make([]byte, fdal.hashSize),
		}
		err = resultSet.Scan(&rowData.Id, &rowData.Hash, &rowData.Path, &rowData.Name, &rowData.NewName)
		if err != nil {
			return nil, err
		}
		resultData = append(resultData, rowData)
	}

	return resultData, nil
}

func (fdal *FimulcrumDbAccessLayer) PersistNewTag(tag *Tag) error {
	fdal.dataMutex.Lock()
	defer fdal.dataMutex.Unlock()
	_, err := fdal.db.Exec(
		fmt.Sprintf("INSERT INTO %s (id, name, comment, actionId) VALUES (?, ?, ?, ?)", fdal.tagsDbName),
		tag.Id,
		tag.Name,
		tag.Comment,
		tag.ActionId,
	)
	return err
}

func (fdal *FimulcrumDbAccessLayer) GetTags(limit uint) ([]*Tag, error) {
	fdal.dataMutex.Lock()
	defer fdal.dataMutex.Unlock()
	resultData := make([]*Tag, 0)
	resultSet, err := fdal.db.Query(
		fmt.Sprintf("SELECT id, name, comment, actionId FROM %s;", fdal.tagsDbName),
	)
	if err != nil {
		return nil, err
	}
	defer resultSet.Close()

	for resultSet.Next() {
		rowData := &Tag{}
		err = resultSet.Scan(&rowData.Id, &rowData.Name, &rowData.Comment, &rowData.ActionId)
		if err != nil {
			return nil, err
		}
		resultData = append(resultData, rowData)
	}

	return resultData, nil
}

func (fdal *FimulcrumDbAccessLayer) CloseDb() error {
	return fdal.db.Close()
}
