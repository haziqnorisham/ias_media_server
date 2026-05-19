package db

import (
	"database/sql"
	"fmt"
)

type Device struct {
	ID                 int64  `json:"id"`
	Name               string `json:"name"`
	IP                 string `json:"ip"`
	Port               int    `json:"port"`
	Username           string `json:"username"`
	Password           string `json:"-"`
	StreamProfileToken string `json:"stream_profile_token"`
	Notes              string `json:"notes,omitempty"`
	CreatedAt          string `json:"created_at"`
	UpdatedAt          string `json:"updated_at"`
}

func InsertDevice(db *sql.DB, name, ip string, port int, username, password string) (*Device, error) {
	row := db.QueryRow(
		`INSERT INTO devices (name, ip, port, username, password) VALUES (?, ?, ?, ?, ?) RETURNING id, name, ip, port, username, password, stream_profile_token, notes, created_at, updated_at`,
		name, ip, port, username, password,
	)
	var d Device
	err := row.Scan(&d.ID, &d.Name, &d.IP, &d.Port, &d.Username, &d.Password, &d.StreamProfileToken, &d.Notes, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert device: %w", err)
	}
	return &d, nil
}

func ListDevices(db *sql.DB) ([]Device, error) {
	rows, err := db.Query(`SELECT id, name, ip, port, username, password, stream_profile_token, notes, created_at, updated_at FROM devices ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list devices: %w", err)
	}
	defer rows.Close()

	var devices []Device
	for rows.Next() {
		var d Device
		if err := rows.Scan(&d.ID, &d.Name, &d.IP, &d.Port, &d.Username, &d.Password, &d.StreamProfileToken, &d.Notes, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan device: %w", err)
		}
		devices = append(devices, d)
	}
	if devices == nil {
		devices = []Device{}
	}
	return devices, rows.Err()
}

func GetDeviceByID(db *sql.DB, id int64) (*Device, error) {
	row := db.QueryRow(`SELECT id, name, ip, port, username, password, stream_profile_token, notes, created_at, updated_at FROM devices WHERE id = ?`, id)
	var d Device
	err := row.Scan(&d.ID, &d.Name, &d.IP, &d.Port, &d.Username, &d.Password, &d.StreamProfileToken, &d.Notes, &d.CreatedAt, &d.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get device: %w", err)
	}
	return &d, nil
}

func UpdateDevice(db *sql.DB, id int64, name, ip string, port int, username, password, streamProfileToken, notes string) (*Device, error) {
	row := db.QueryRow(
		`UPDATE devices SET name=?, ip=?, port=?, username=?, password=?, stream_profile_token=?, notes=?, updated_at=datetime('now') WHERE id=? RETURNING id, name, ip, port, username, password, stream_profile_token, notes, created_at, updated_at`,
		name, ip, port, username, password, streamProfileToken, notes, id,
	)
	var d Device
	err := row.Scan(&d.ID, &d.Name, &d.IP, &d.Port, &d.Username, &d.Password, &d.StreamProfileToken, &d.Notes, &d.CreatedAt, &d.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("update device: %w", err)
	}
	return &d, nil
}

func SetStreamProfile(db *sql.DB, deviceID int64, token string) error {
	_, err := db.Exec(`UPDATE devices SET stream_profile_token=?, updated_at=datetime('now') WHERE id=?`, token, deviceID)
	return err
}

func DeleteDevice(db *sql.DB, id int64) (bool, error) {
	result, err := db.Exec(`DELETE FROM devices WHERE id = ?`, id)
	if err != nil {
		return false, fmt.Errorf("delete device: %w", err)
	}
	n, err := result.RowsAffected()
	return n > 0, err
}
