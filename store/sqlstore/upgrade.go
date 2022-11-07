// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package sqlstore

import (
	"database/sql"
)

type upgradeFunc func(*sql.Tx, *Container) error

// Upgrades is a list of functions that will upgrade a database to the latest version.
//
// This may be of use if you want to manage the database fully manually, but in most cases you
// should just call Container.Upgrade to let the library handle everything.
var Upgrades = [...]upgradeFunc{upgradeV1, upgradeV2}

func (c *Container) getVersion() (int, error) {
	_, err := c.db.Exec("CREATE TABLE IF NOT EXISTS whatsapp_version (version INTEGER)")
	if err != nil {
		return -1, err
	}

	version := 0
	row := c.db.QueryRow("SELECT version FROM whatsapp_version LIMIT 1")
	if row != nil {
		_ = row.Scan(&version)
	}
	return version, nil
}

func (c *Container) setVersion(tx *sql.Tx, version int) error {
	_, err := tx.Exec("DELETE FROM whatsapp_version")
	if err != nil {
		return err
	}
	_, err = tx.Exec("INSERT INTO whatsapp_version (version) VALUES ($1)", version)
	return err
}

// Upgrade upgrades the database from the current to the latest version available.
func (c *Container) Upgrade() error {
	version, err := c.getVersion()
	if err != nil {
		return err
	}

	for ; version < len(Upgrades); version++ {
		var tx *sql.Tx
		tx, err = c.db.Begin()
		if err != nil {
			return err
		}

		migrateFunc := Upgrades[version]
		c.log.Infof("Upgrading database to v%d", version+1)
		err = migrateFunc(tx, c)
		if err != nil {
			_ = tx.Rollback()
			return err
		}

		if err = c.setVersion(tx, version+1); err != nil {
			return err
		}

		if err = tx.Commit(); err != nil {
			return err
		}
	}

	return nil
}

func upgradeV1(tx *sql.Tx, _ *Container) error {
	_, err := tx.Exec(`CREATE TABLE whatsapp_device (
		jid TEXT PRIMARY KEY,

		registration_id BIGINT NOT NULL CHECK ( registration_id >= 0 AND registration_id < 4294967296 ),

		noise_key    bytea NOT NULL CHECK ( length(noise_key) = 32 ),
		identity_key bytea NOT NULL CHECK ( length(identity_key) = 32 ),

		signed_pre_key     bytea   NOT NULL CHECK ( length(signed_pre_key) = 32 ),
		signed_pre_key_id  INTEGER NOT NULL CHECK ( signed_pre_key_id >= 0 AND signed_pre_key_id < 16777216 ),
		signed_pre_key_sig bytea   NOT NULL CHECK ( length(signed_pre_key_sig) = 64 ),

		adv_key         bytea NOT NULL,
		adv_details     bytea NOT NULL,
		adv_account_sig bytea NOT NULL CHECK ( length(adv_account_sig) = 64 ),
		adv_device_sig  bytea NOT NULL CHECK ( length(adv_device_sig) = 64 ),

		platform      TEXT NOT NULL DEFAULT '',
		business_name TEXT NOT NULL DEFAULT '',
		push_name     TEXT NOT NULL DEFAULT ''
	)`)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`CREATE TABLE whatsapp_identity_keys (
		our_jid  TEXT,
		their_id TEXT,
		identity bytea NOT NULL CHECK ( length(identity) = 32 ),

		PRIMARY KEY (our_jid, their_id),
		FOREIGN KEY (our_jid) REFERENCES whatsapp_device(jid) ON DELETE CASCADE ON UPDATE CASCADE
	)`)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`CREATE TABLE whatsapp_pre_keys (
		jid      TEXT,
		key_id   INTEGER          CHECK ( key_id >= 0 AND key_id < 16777216 ),
		key      bytea   NOT NULL CHECK ( length(key) = 32 ),
		uploaded BOOLEAN NOT NULL,

		PRIMARY KEY (jid, key_id),
		FOREIGN KEY (jid) REFERENCES whatsapp_device(jid) ON DELETE CASCADE ON UPDATE CASCADE
	)`)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`CREATE TABLE whatsapp_sessions (
		our_jid  TEXT,
		their_id TEXT,
		session  bytea,

		PRIMARY KEY (our_jid, their_id),
		FOREIGN KEY (our_jid) REFERENCES whatsapp_device(jid) ON DELETE CASCADE ON UPDATE CASCADE
	)`)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`CREATE TABLE whatsapp_sender_keys (
		our_jid    TEXT,
		chat_id    TEXT,
		sender_id  TEXT,
		sender_key bytea NOT NULL,

		PRIMARY KEY (our_jid, chat_id, sender_id),
		FOREIGN KEY (our_jid) REFERENCES whatsapp_device(jid) ON DELETE CASCADE ON UPDATE CASCADE
	)`)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`CREATE TABLE whatsapp_app_state_sync_keys (
		jid         TEXT,
		key_id      bytea,
		key_data    bytea  NOT NULL,
		timestamp   BIGINT NOT NULL,
		fingerprint bytea  NOT NULL,

		PRIMARY KEY (jid, key_id),
		FOREIGN KEY (jid) REFERENCES whatsapp_device(jid) ON DELETE CASCADE ON UPDATE CASCADE
	)`)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`CREATE TABLE whatsapp_app_state_version (
		jid     TEXT,
		name    TEXT,
		version BIGINT NOT NULL,
		hash    bytea  NOT NULL CHECK ( length(hash) = 128 ),

		PRIMARY KEY (jid, name),
		FOREIGN KEY (jid) REFERENCES whatsapp_device(jid) ON DELETE CASCADE ON UPDATE CASCADE
	)`)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`CREATE TABLE whatsapp_app_state_mutation_macs (
		jid       TEXT,
		name      TEXT,
		version   BIGINT,
		index_mac bytea          CHECK ( length(index_mac) = 32 ),
		value_mac bytea NOT NULL CHECK ( length(value_mac) = 32 ),

		PRIMARY KEY (jid, name, version, index_mac),
		FOREIGN KEY (jid, name) REFERENCES whatsapp_app_state_version(jid, name) ON DELETE CASCADE ON UPDATE CASCADE
	)`)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`CREATE TABLE whatsapp_contacts (
		our_jid       TEXT,
		their_jid     TEXT,
		first_name    TEXT,
		full_name     TEXT,
		push_name     TEXT,
		business_name TEXT,

		PRIMARY KEY (our_jid, their_jid),
		FOREIGN KEY (our_jid) REFERENCES whatsapp_device(jid) ON DELETE CASCADE ON UPDATE CASCADE
	)`)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`CREATE TABLE whatsapp_chat_settings (
		our_jid       TEXT,
		chat_jid      TEXT,
		muted_until   BIGINT  NOT NULL DEFAULT 0,
		pinned        BOOLEAN NOT NULL DEFAULT false,
		archived      BOOLEAN NOT NULL DEFAULT false,

		PRIMARY KEY (our_jid, chat_jid),
		FOREIGN KEY (our_jid) REFERENCES whatsapp_device(jid) ON DELETE CASCADE ON UPDATE CASCADE
	)`)
	if err != nil {
		return err
	}
	return nil
}

const fillSigKeyPostgres = `
UPDATE whatsapp_device SET adv_account_sig_key=(
	SELECT identity
	FROM whatsapp_identity_keys
	WHERE our_jid=whatsapp_device.jid
	  AND their_id=concat(split_part(whatsapp_device.jid, '.', 1), ':0')
);
DELETE FROM whatsapp_device WHERE adv_account_sig_key IS NULL;
ALTER TABLE whatsapp_device ALTER COLUMN adv_account_sig_key SET NOT NULL;
`

const fillSigKeySQLite = `
UPDATE whatsapp_device SET adv_account_sig_key=(
	SELECT identity
	FROM whatsapp_identity_keys
	WHERE our_jid=whatsapp_device.jid
	  AND their_id=substr(whatsapp_device.jid, 0, instr(whatsapp_device.jid, '.')) || ':0'
)
`

func upgradeV2(tx *sql.Tx, container *Container) error {
	_, err := tx.Exec("ALTER TABLE whatsapp_device ADD COLUMN adv_account_sig_key bytea CHECK ( length(adv_account_sig_key) = 32 )")
	if err != nil {
		return err
	}
	if container.dialect == "postgres" || container.dialect == "pgx" {
		_, err = tx.Exec(fillSigKeyPostgres)
	} else {
		_, err = tx.Exec(fillSigKeySQLite)
	}
	return err
}
