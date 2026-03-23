package vectorstore

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	gormsqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var autoOnce sync.Once

type Match struct {
	RowID    uint
	Distance float64
}

func Open(path string) (*gorm.DB, error) {
	autoOnce.Do(sqlite_vec.Auto)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create database dir: %w", err)
	}

	sqlDB, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	if err := verifyExtension(sqlDB); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}

	db, err := gorm.Open(gormsqlite.New(gormsqlite.Config{Conn: sqlDB}), &gorm.Config{})
	if err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("open gorm database: %w", err)
	}

	return db, nil
}

func EnsureSchema(db *gorm.DB, model string, dimensions int) error {
	if dimensions <= 0 {
		dimensions = 2560
	}

	if err := db.Exec(`create table if not exists embedding_config (key text primary key, value text not null)`).Error; err != nil {
		return fmt.Errorf("create embedding config table: %w", err)
	}

	if err := ensureConfigValue(db, "embedding_dimensions", fmt.Sprintf("%d", dimensions)); err != nil {
		return err
	}
	if model != "" {
		if err := ensureConfigValue(db, "embedding_model", model); err != nil {
			return err
		}
	}

	if err := db.Exec(fmt.Sprintf(`create virtual table if not exists page_embeddings using vec0(embedding float[%d] distance_metric=cosine)`, dimensions)).Error; err != nil {
		return fmt.Errorf("create page embeddings table: %w", err)
	}

	return nil
}

func DeleteEmbeddings(db *gorm.DB, rowIDs []uint) error {
	if len(rowIDs) == 0 {
		return nil
	}
	args := make([]any, 0, len(rowIDs))
	placeholders := make([]byte, 0, len(rowIDs)*2)
	for i, rowID := range rowIDs {
		if i > 0 {
			placeholders = append(placeholders, ',')
		}
		placeholders = append(placeholders, '?')
		args = append(args, rowID)
	}

	if err := db.Exec(`delete from page_embeddings where rowid in (`+string(placeholders)+`)`, args...).Error; err != nil {
		return fmt.Errorf("delete page embeddings: %w", err)
	}

	return nil
}

func UpsertEmbedding(db *gorm.DB, rowID uint, vector []float32) error {
	if len(vector) == 0 {
		return fmt.Errorf("embedding vector is empty")
	}
	blob, err := sqlite_vec.SerializeFloat32(vector)
	if err != nil {
		return fmt.Errorf("serialize page embedding: %w", err)
	}

	if err := db.Exec(`insert or replace into page_embeddings(rowid, embedding) values (?, ?)`, rowID, blob).Error; err != nil {
		return fmt.Errorf("upsert page embedding: %w", err)
	}

	return nil
}

func Search(db *gorm.DB, queryVector []float32, limit int) ([]Match, error) {
	if len(queryVector) == 0 {
		return nil, nil
	}
	if limit <= 0 {
		limit = 20
	}
	blob, err := sqlite_vec.SerializeFloat32(queryVector)
	if err != nil {
		return nil, fmt.Errorf("serialize query vector: %w", err)
	}

	rows, err := db.Raw(`select rowid, distance from page_embeddings where embedding match ? and k = ? order by distance`, blob, limit).Rows()
	if err != nil {
		return nil, fmt.Errorf("search page embeddings: %w", err)
	}
	defer rows.Close()

	var matches []Match
	for rows.Next() {
		var match Match
		if err := rows.Scan(&match.RowID, &match.Distance); err != nil {
			return nil, fmt.Errorf("scan page embedding match: %w", err)
		}
		matches = append(matches, match)
	}

	return matches, rows.Err()
}

func verifyExtension(db *sql.DB) error {
	var version string
	if err := db.QueryRow(`select vec_version()`).Scan(&version); err != nil {
		return fmt.Errorf("verify sqlite-vec extension: %w", err)
	}
	return nil
}

func ensureConfigValue(db *gorm.DB, key string, want string) error {
	type configRow struct {
		Value string
	}

	var row configRow
	result := db.Raw(`select value from embedding_config where key = ?`, key).Scan(&row)
	if result.Error != nil {
		return fmt.Errorf("read embedding config %q: %w", key, result.Error)
	}
	if result.RowsAffected == 0 {
		if err := db.Exec(`insert into embedding_config(key, value) values(?, ?)`, key, want).Error; err != nil {
			return fmt.Errorf("write embedding config %q: %w", key, err)
		}
		return nil
	}
	if row.Value != want {
		return fmt.Errorf("embedding config mismatch for %s: database=%q config=%q", key, row.Value, want)
	}
	return nil
}
