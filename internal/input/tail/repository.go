package inputtail

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/MuchTitan/go-log-forwarder/internal/database"
)

type TailRepository interface {
	CreateTables() error
	GetFileState(path string, inode uint64) (*fileState, error)
	DeleteFileState(path string, inode uint64) error
	BatchUpsertFileStates(states []fileState) error
	Close() error
}

type SQLiteTailRepository struct {
	db *database.DBManager
}

func NewSQLiteTailRepository(dbFile string) TailRepository {
	dbManager, err := database.NewDBManager(dbFile)
	if err != nil {
		return nil
	}
	return &SQLiteTailRepository{
		db: dbManager,
	}
}

func (r *SQLiteTailRepository) CreateTables() error {
	query := `CREATE TABLE IF NOT EXISTS tail_files (
        path TEXT NOT NULL,
        offset INTEGER NOT NULL,
        lastReadLine INTEGER NOT NULL,
        inodenumber INTEGER NOT NULL,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        PRIMARY KEY (path, inodenumber)
    )`
	_, err := r.db.ExecuteWrite(query)
	if err != nil {
		return fmt.Errorf("could not create db table tail_files: %v", err)
	}
	return nil
}

func (r *SQLiteTailRepository) UpsertFileState(state *fileState) error {
	query := `
        INSERT OR REPLACE INTO tail_files 
        (path, offset, lastReadLine, inodenumber, updated_at) 
        VALUES ($1, $2, $3, $4, $5)`

	_, err := r.db.ExecuteWrite(query,
		state.Path,
		state.Offset,
		state.LastReadLine,
		state.InodeNumber,
		time.Now(),
	)
	return err
}

func (r *SQLiteTailRepository) BatchUpsertFileStates(states []fileState) error {
	return r.db.ExecuteWriteTx(func(tx *sql.Tx) error {
		stmt, err := tx.Prepare(`
            INSERT OR REPLACE INTO tail_files
            (path, offset, lastReadLine, inodenumber, updated_at)
            VALUES ($1, $2, $3, $4, $5)
        `)
		if err != nil {
			return err
		}
		defer stmt.Close()

		for _, state := range states {
			_, err := stmt.Exec(
				state.Path,
				state.Offset,
				state.LastReadLine,
				state.InodeNumber,
				time.Now(),
			)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *SQLiteTailRepository) GetFileState(path string, inode uint64) (*fileState, error) {
	query := `SELECT path, offset, lastReadLine, inodenumber, created_at, updated_at 
              FROM tail_files 
              WHERE path = $1 AND inodenumber = $2`

	row := r.db.QueryRow(query, path, inode)

	state := &fileState{}
	err := row.Scan(
		&state.Path,
		&state.Offset,
		&state.LastReadLine,
		&state.InodeNumber,
		&state.CreatedAt,
		&state.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return state, nil
}

func (r *SQLiteTailRepository) DeleteFileState(path string, inode uint64) error {
	query := `DELETE FROM tail_files WHERE path = $1 AND inodenumber = $2`
	_, err := r.db.ExecuteWrite(query, path, inode)
	return err
}

func (r *SQLiteTailRepository) Close() error {
	if err := r.db.Close(); err != nil {
		return err
	}
	return nil
}
