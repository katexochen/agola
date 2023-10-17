package db

import (
	"github.com/sorintlab/errors"

	"agola.io/agola/internal/sqlg"
	"agola.io/agola/internal/sqlg/sql"
)

func (d *DB) MigrateFuncs() map[uint]sqlg.MigrateFunc {
	return map[uint]sqlg.MigrateFunc{
		2: d.migrateV2,
		3: d.migrateV3,
	}
}

func (d *DB) migrateV2(tx *sql.Tx) error {
	var ddlPostgres = []string{
		"create table if not exists lastruneventsequence (id varchar NOT NULL, revision bigint NOT NULL, creation_time timestamptz NOT NULL, update_time timestamptz NOT NULL, value bigint NOT NULL, PRIMARY KEY (id))",
	}

	var ddlSqlite3 = []string{
		"create table if not exists lastruneventsequence (id varchar NOT NULL, revision bigint NOT NULL, creation_time timestamp NOT NULL, update_time timestamp NOT NULL, value bigint NOT NULL, PRIMARY KEY (id))",
	}

	var stmts []string
	switch d.sdb.Type() {
	case sql.Postgres:
		stmts = ddlPostgres
	case sql.Sqlite3:
		stmts = ddlSqlite3
	}

	for _, stmt := range stmts {
		if _, err := tx.Exec(stmt); err != nil {
			return errors.WithStack(err)
		}
	}

	return nil
}

func (d *DB) migrateV3(tx *sql.Tx) error {
	var ddlPostgres = []string{
		"create table if not exists commitstatus (id varchar NOT NULL, revision bigint NOT NULL, creation_time timestamptz NOT NULL, update_time timestamptz NOT NULL, project_id varchar NOT NULL, state varchar NOT NULL, commit_sha varchar NOT NULL, run_counter bigint NOT NULL, description varchar NOT NULL, context varchar NOT NULL, PRIMARY KEY (id))",
		"create table if not exists commitstatusdelivery (id varchar NOT NULL, revision bigint NOT NULL, creation_time timestamptz NOT NULL, update_time timestamptz NOT NULL, sequence bigint generated by default as identity NOT NULL UNIQUE, commit_status_id varchar NOT NULL, delivery_status varchar NOT NULL, delivered_at timestamptz, PRIMARY KEY (id), foreign key (commit_status_id) references commitstatus(id))",
		"create index if not exists commitstatusdelivery_sequence_idx on commitstatusdelivery(sequence)",
	}

	var ddlSqlite3 = []string{
		"create table if not exists commitstatus (id varchar NOT NULL, revision bigint NOT NULL, creation_time timestamp NOT NULL, update_time timestamp NOT NULL, project_id varchar NOT NULL, state varchar NOT NULL, commit_sha varchar NOT NULL, run_counter bigint NOT NULL, description varchar NOT NULL, context varchar NOT NULL, PRIMARY KEY (id))",
		"create table if not exists commitstatusdelivery (id varchar NOT NULL, revision bigint NOT NULL, creation_time timestamp NOT NULL, update_time timestamp NOT NULL, sequence integer NOT NULL UNIQUE, commit_status_id varchar NOT NULL, delivery_status varchar NOT NULL, delivered_at timestamp, PRIMARY KEY (id), foreign key (commit_status_id) references commitstatus(id))",
		"create index if not exists commitstatusdelivery_sequence_idx on commitstatusdelivery(sequence)",
	}

	var stmts []string
	switch d.sdb.Type() {
	case sql.Postgres:
		stmts = ddlPostgres
	case sql.Sqlite3:
		stmts = ddlSqlite3
	}

	for _, stmt := range stmts {
		if _, err := tx.Exec(stmt); err != nil {
			return errors.WithStack(err)
		}
	}

	return nil
}