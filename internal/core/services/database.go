package services

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/flevanti/airflow-migrator/internal/core/models"
	_ "github.com/lib/pq" // PostgreSQL driver
)

// Database provides operations on Airflow's metadata database.
type Database struct {
	db *sql.DB
}

// NewDatabase creates a new database connection.
func NewDatabase(profile *models.Profile) (*Database, error) {
	db, err := sql.Open("postgres", profile.DSN())
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	return &Database{db: db}, nil
}

// Close closes the database connection.
func (d *Database) Close() error {
	return d.db.Close()
}

// TestConnection tests the database connection.
func (d *Database) TestConnection(ctx context.Context) error {
	return d.db.PingContext(ctx)
}

// ListConnections retrieves all connections from the Airflow database.
func (d *Database) ListConnections(ctx context.Context) ([]*models.Connection, error) {
	query := `
		SELECT conn_id, conn_type, description, host, schema, login, password, port, extra
		FROM connection
		ORDER BY conn_id
	`

	rows, err := d.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query connections: %w", err)
	}
	defer rows.Close()

	var connections []*models.Connection
	for rows.Next() {
		conn := &models.Connection{}
		var description, host, schema, login, password, extra sql.NullString
		var port sql.NullInt32

		err := rows.Scan(
			&conn.ID,
			&conn.ConnType,
			&description,
			&host,
			&schema,
			&login,
			&password,
			&port,
			&extra,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		conn.Description = description.String
		conn.Host = host.String
		conn.Schema = schema.String
		conn.Login = login.String
		conn.Password = password.String
		conn.Port = int(port.Int32)
		conn.Extra = extra.String
		conn.IsEncrypted = password.Valid && password.String != ""

		connections = append(connections, conn)
	}

	return connections, rows.Err()
}

// GetConnection retrieves a single connection by ID.
func (d *Database) GetConnection(ctx context.Context, connID string) (*models.Connection, error) {
	query := `
		SELECT conn_id, conn_type, description, host, schema, login, password, port, extra
		FROM connection
		WHERE conn_id = $1
	`

	conn := &models.Connection{}
	var description, host, schema, login, password, extra sql.NullString
	var port sql.NullInt32

	err := d.db.QueryRowContext(ctx, query, connID).Scan(
		&conn.ID,
		&conn.ConnType,
		&description,
		&host,
		&schema,
		&login,
		&password,
		&port,
		&extra,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}

	conn.Description = description.String
	conn.Host = host.String
	conn.Schema = schema.String
	conn.Login = login.String
	conn.Password = password.String
	conn.Port = int(port.Int32)
	conn.Extra = extra.String

	return conn, nil
}

// ConnectionExists checks if a connection ID exists.
func (d *Database) ConnectionExists(ctx context.Context, connID string) (bool, error) {
	var exists bool
	err := d.db.QueryRowContext(ctx,
		"SELECT EXISTS(SELECT 1 FROM connection WHERE conn_id = $1)",
		connID,
	).Scan(&exists)
	return exists, err
}

// InsertConnection inserts a new connection.
func (d *Database) InsertConnection(ctx context.Context, conn *models.Connection) error {
	query := `
		INSERT INTO connection (conn_id, conn_type, description, host, schema, login, password, port, extra)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err := d.db.ExecContext(ctx, query,
		conn.ID,
		conn.ConnType,
		nullString(conn.Description),
		nullString(conn.Host),
		nullString(conn.Schema),
		nullString(conn.Login),
		nullString(conn.Password),
		nullInt(conn.Port),
		nullString(conn.Extra),
	)
	if err != nil {
		return fmt.Errorf("failed to insert connection: %w", err)
	}

	return nil
}

// UpdateConnection updates an existing connection.
func (d *Database) UpdateConnection(ctx context.Context, conn *models.Connection) error {
	query := `
		UPDATE connection
		SET conn_type = $2, description = $3, host = $4, schema = $5,
		    login = $6, password = $7, port = $8, extra = $9
		WHERE conn_id = $1
	`

	result, err := d.db.ExecContext(ctx, query,
		conn.ID,
		conn.ConnType,
		nullString(conn.Description),
		nullString(conn.Host),
		nullString(conn.Schema),
		nullString(conn.Login),
		nullString(conn.Password),
		nullInt(conn.Port),
		nullString(conn.Extra),
	)
	if err != nil {
		return fmt.Errorf("failed to update connection: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("connection not found: %s", conn.ID)
	}

	return nil
}

// DeleteConnection deletes a connection by ID.
func (d *Database) DeleteConnection(ctx context.Context, connID string) error {
	result, err := d.db.ExecContext(ctx, "DELETE FROM connection WHERE conn_id = $1", connID)
	if err != nil {
		return fmt.Errorf("failed to delete connection: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("connection not found: %s", connID)
	}

	return nil
}

// GetExistingConnectionIDs returns IDs that already exist from a given list.
func (d *Database) GetExistingConnectionIDs(ctx context.Context, ids []string) ([]string, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	// Build placeholder string: $1, $2, $3...
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	query := fmt.Sprintf(
		"SELECT conn_id FROM connection WHERE conn_id IN (%s)",
		strings.Join(placeholders, ", "),
	)

	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var existing []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		existing = append(existing, id)
	}

	return existing, rows.Err()
}

// Helper functions for nullable fields
func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func nullInt(i int) sql.NullInt32 {
	if i == 0 {
		return sql.NullInt32{}
	}
	return sql.NullInt32{Int32: int32(i), Valid: true}
}
