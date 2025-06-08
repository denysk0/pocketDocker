package store

import (
	"time"
)

// ImageInfo holds metadata about an image
// ID is derived from name (not unique?). For simplicity we use name as ID
// but in future may use uuid.
type ImageInfo struct {
	Name      string
	Path      string
	CreatedAt time.Time
}

// SaveImage inserts or updates image metadata
func (s *Store) SaveImage(info ImageInfo) error {
	_, err := s.db.Exec(`INSERT INTO images(name, path, created_at) VALUES (?, ?, ?) ON CONFLICT(name) DO UPDATE SET path=excluded.path, created_at=excluded.created_at`,
		info.Name, info.Path, info.CreatedAt.Format(time.RFC3339))
	return err
}

// GetImage fetches image metadata by name
func (s *Store) GetImage(name string) (ImageInfo, error) {
	row := s.db.QueryRow(`SELECT name, path, created_at FROM images WHERE name = ?`, name)
	var info ImageInfo
	var t string
	if err := row.Scan(&info.Name, &info.Path, &t); err != nil {
		return ImageInfo{}, err
	}
	info.CreatedAt, _ = time.Parse(time.RFC3339, t)
	return info, nil
}

// ListImages returns all stored images
func (s *Store) ListImages() ([]ImageInfo, error) {
	rows, err := s.db.Query(`SELECT name, path, created_at FROM images`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ImageInfo
	for rows.Next() {
		var info ImageInfo
		var t string
		if err := rows.Scan(&info.Name, &info.Path, &t); err != nil {
			return nil, err
		}
		info.CreatedAt, _ = time.Parse(time.RFC3339, t)
		out = append(out, info)
	}
	return out, rows.Err()
}
