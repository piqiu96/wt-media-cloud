package identity

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
)

type MySQLStore struct {
	db *sql.DB
}

func NewMySQLStore(db *sql.DB) *MySQLStore {
	return &MySQLStore{db: db}
}

func (s *MySQLStore) CountUsers() (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count)
	return count, err
}

func (s *MySQLStore) CreateUser(user User) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(
		`INSERT INTO users (id, username, password_hash, role, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		user.ID, user.Username, user.PasswordHash, user.Role, user.Status, user.CreatedAt, user.UpdatedAt,
	)
	if err != nil {
		if duplicateKey(err) {
			return ErrUsernameTaken
		}
		return err
	}
	for _, gameID := range user.GameIDs {
		if _, err := tx.Exec(
			`INSERT INTO user_game_scopes (user_id, game_id, created_at) VALUES (?, ?, ?)`,
			user.ID, gameID, user.CreatedAt,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *MySQLStore) FindUser(id string) (User, bool, error) {
	return s.findUser(`SELECT id, username, password_hash, role, status, created_at, updated_at FROM users WHERE id = ?`, id)
}

func (s *MySQLStore) FindUserByUsername(username string) (User, bool, error) {
	return s.findUser(`SELECT id, username, password_hash, role, status, created_at, updated_at FROM users WHERE username = ?`, username)
}

func (s *MySQLStore) findUser(query string, arg string) (User, bool, error) {
	var user User
	err := s.db.QueryRow(query, arg).Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&user.Role,
		&user.Status,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, false, nil
	}
	if err != nil {
		return User{}, false, err
	}
	gameIDs, err := s.findGameIDs(user.ID)
	if err != nil {
		return User{}, false, err
	}
	user.GameIDs = gameIDs
	return user, true, nil
}

func (s *MySQLStore) findGameIDs(userID string) ([]string, error) {
	rows, err := s.db.Query(`SELECT game_id FROM user_game_scopes WHERE user_id = ? ORDER BY game_id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var gameIDs []string
	for rows.Next() {
		var gameID string
		if err := rows.Scan(&gameID); err != nil {
			return nil, err
		}
		gameIDs = append(gameIDs, gameID)
	}
	return gameIDs, rows.Err()
}

func (s *MySQLStore) UpdateUser(user User) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	result, err := tx.Exec(
		`UPDATE users SET username = ?, password_hash = ?, role = ?, status = ?, updated_at = ? WHERE id = ?`,
		user.Username, user.PasswordHash, user.Role, user.Status, user.UpdatedAt, user.ID,
	)
	if err != nil {
		if duplicateKey(err) {
			return ErrUsernameTaken
		}
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrInvalidInput
	}
	if _, err := tx.Exec(`DELETE FROM user_game_scopes WHERE user_id = ?`, user.ID); err != nil {
		return err
	}
	for _, gameID := range user.GameIDs {
		if _, err := tx.Exec(
			`INSERT INTO user_game_scopes (user_id, game_id, created_at) VALUES (?, ?, ?)`,
			user.ID, gameID, user.UpdatedAt,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *MySQLStore) CreateSession(session Session) error {
	_, err := s.db.Exec(
		`INSERT INTO user_sessions (id, user_id, token_hash, created_at, invalidated_at) VALUES (?, ?, ?, ?, ?)`,
		session.ID, session.UserID, session.TokenHash, session.CreatedAt, session.InvalidAt,
	)
	return err
}

func (s *MySQLStore) FindSessionByTokenHash(tokenHash string) (Session, bool, error) {
	var session Session
	var invalidatedAt sql.NullTime
	err := s.db.QueryRow(
		`SELECT id, user_id, token_hash, created_at, invalidated_at FROM user_sessions WHERE token_hash = ?`,
		tokenHash,
	).Scan(&session.ID, &session.UserID, &session.TokenHash, &session.CreatedAt, &invalidatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, false, nil
	}
	if err != nil {
		return Session{}, false, err
	}
	if invalidatedAt.Valid {
		at := invalidatedAt.Time
		session.InvalidAt = &at
	}
	return session, true, nil
}

func (s *MySQLStore) InvalidateUserSessions(userID string, at time.Time) error {
	_, err := s.db.Exec(
		`UPDATE user_sessions SET invalidated_at = ? WHERE user_id = ? AND invalidated_at IS NULL`,
		at, userID,
	)
	return err
}

func (s *MySQLStore) AppendAudit(event AuditEvent) error {
	summary, err := json.Marshal(event.Summary)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(
		`INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id, summary_json, created_at) VALUES (?, NULLIF(?, ''), ?, ?, NULLIF(?, ''), ?, ?)`,
		event.ID, event.ActorUserID, event.Action, event.TargetType, event.TargetID, summary, event.CreatedAt,
	)
	return err
}

func duplicateKey(err error) bool {
	var mysqlErr *mysqlDriver.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}
