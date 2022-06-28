package main

import (
	"encoding/json"
	"epa/wclogs"
	"strconv"

	"github.com/tidwall/buntdb"
)

func fetchWCLogsCredentials(db *buntdb.DB, guildID string) (*wclogs.Credentials, error) {
	creds := &wclogs.Credentials{}
	err := db.View(func(tx *buntdb.Tx) error {
		val, err := tx.Get("wclogs-creds-" + guildID)
		if err != nil {
			return err
		}

		return json.Unmarshal([]byte(val), creds)
	})

	if err != nil {
		return nil, err
	}

	return creds, nil
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

func fetchWCLogsLatestReportForCharacter(db *buntdb.DB, charID int) (*wclogs.Report, error) {
	report := &wclogs.Report{}
	err := db.View(func(tx *buntdb.Tx) error {
		val, err := tx.Get("wclogs-latest-report-" + strconv.Itoa(charID))
		if err != nil {
			return err
		}

		return json.Unmarshal([]byte(val), report)
	})

	if err != nil {
		return nil, err
	}

	return report, nil
}

func storeWCLogsLatestReportForCharacter(db *buntdb.DB, charID int, report *wclogs.Report) error {
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

func fetchWCLogsTrackedCharacters(db *buntdb.DB, guildID string) ([]int, error) {
	characters := &[]int{}
	err := db.View(func(tx *buntdb.Tx) error {
		val, err := tx.Get("wclogs-tracked-characters-" + guildID)
		if err != nil {
			return err
		}

		return json.Unmarshal([]byte(val), characters)
	})

	if err != nil {
		return nil, err
	}

	return *characters, nil
}

func storeWCLogsTrackedCharacters(db *buntdb.DB, guildID string, characters []int) error {
	bytes, err := json.Marshal(characters)
	if err != nil {
		return err
	}

	err = db.Update(func(tx *buntdb.Tx) error {
		_, _, err := tx.Set("wclogs-tracked-characters-"+guildID, string(bytes), nil)
		return err
	})

	return err
}
