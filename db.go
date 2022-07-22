package main

import (
	"encoding/json"
	"strconv"

	"github.com/tidwall/buntdb"
	"github.com/zergrael/epa/wclogs"
)

const currentDatabaseVersion = 1

// upgradeDatabaseIfNecessary checks database version and tries to migrate if necessary
func upgradeDatabaseIfNecessary() error {
	dbVersion := 0

	err := db.View(func(tx *buntdb.Tx) error {
		val, err := tx.Get("version")
		// Ignore not-found errors
		if err == nil {
			dbVersion, err = strconv.Atoi(val)
		}
		return err
	})
	if err != nil {
		return err
	}

	if dbVersion >= currentDatabaseVersion {
		return nil
	}

	// Migration time !
	switch dbVersion {
	case 0:
		// Basically delete everything
		db.Update(func(tx *buntdb.Tx) error {
			return tx.DeleteAll()
		})
		fallthrough
	case 1:
		// Current version
	}

	db.Update(func(tx *buntdb.Tx) error {
		_, _, err := tx.Set("version", strconv.Itoa(currentDatabaseVersion), nil)
		return err
	})

	return nil
}

func fetchWCLogsCredentials(db *buntdb.DB, guildID string) (*wclogs.Credentials, error) {
	var creds wclogs.Credentials
	err := db.View(func(tx *buntdb.Tx) error {
		val, err := tx.Get("wclogs-creds-" + guildID)
		if err != nil {
			return err
		}

		return json.Unmarshal([]byte(val), &creds)
	})

	if err != nil {
		return nil, err
	}

	return &creds, nil
}

func storeWCLogsCredentials(db *buntdb.DB, guildID string, creds *wclogs.Credentials) error {
	bytes, err := json.Marshal(creds)
	if err != nil {
		return err
	}

	err = db.Update(func(tx *buntdb.Tx) error {
		_, _, err := tx.Set("wclogs-creds-"+guildID, string(bytes), nil)
		return err
	})

	return err
}

func fetchWCLogsLatestReportForCharacterID(db *buntdb.DB, charID int) (*wclogs.Report, error) {
	var report wclogs.Report
	err := db.View(func(tx *buntdb.Tx) error {
		val, err := tx.Get("wclogs-latest-report-" + strconv.Itoa(charID))
		if err != nil {
			return err
		}

		return json.Unmarshal([]byte(val), &report)
	})

	if err != nil {
		return nil, err
	}

	return &report, nil
}

func storeWCLogsLatestReportForCharacterID(db *buntdb.DB, charID int, report *wclogs.Report) error {
	bytes, err := json.Marshal(report)
	if err != nil {
		return err
	}

	err = db.Update(func(tx *buntdb.Tx) error {
		_, _, err := tx.Set("wclogs-latest-report-"+strconv.Itoa(charID), string(bytes), nil)
		return err
	})

	return err
}

func fetchWCLogsTrackedCharacters(db *buntdb.DB, guildID string) (*[]TrackedCharacter, error) {
	var characters []TrackedCharacter
	err := db.View(func(tx *buntdb.Tx) error {
		val, err := tx.Get("wclogs-tracked-characters-" + guildID)
		if err != nil {
			return err
		}

		return json.Unmarshal([]byte(val), &characters)
	})

	if err != nil {
		return nil, err
	}

	return &characters, nil
}

func storeWCLogsTrackedCharacters(db *buntdb.DB, guildID string, characters *[]TrackedCharacter) error {
	bytes, err := json.Marshal(*characters)
	if err != nil {
		return err
	}

	err = db.Update(func(tx *buntdb.Tx) error {
		_, _, err := tx.Set("wclogs-tracked-characters-"+guildID, string(bytes), nil)
		return err
	})

	return err
}

func fetchWCLogsParsesForCharacterID(db *buntdb.DB, charID int) (*wclogs.Parses, error) {
	var parses wclogs.Parses
	err := db.View(func(tx *buntdb.Tx) error {
		val, err := tx.Get("wclogs-parses-" + strconv.Itoa(charID))
		if err != nil {
			return err
		}

		return json.Unmarshal([]byte(val), &parses)
	})

	if err != nil {
		return nil, err
	}

	return &parses, nil
}

func storeWCLogsParsesForCharacterID(db *buntdb.DB, charID int, parses *wclogs.Parses) error {
	bytes, err := json.Marshal(parses)
	if err != nil {
		return err
	}

	err = db.Update(func(tx *buntdb.Tx) error {
		_, _, err := tx.Set("wclogs-parses-"+strconv.Itoa(charID), string(bytes), nil)
		return err
	})

	return err
}
