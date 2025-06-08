package store

import (
	"database/sql"
	"time"

	_ "modernc.org/sqlite"
)

type ContainerInfo struct {
	ID        string
	Name      string
	Image     string
	PID       int
	State     string
	StartedAt time.Time
	RootfsDir string
}

type Store struct {
	db *sql.DB
}

func NewStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Init() error {
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS containers (
        id TEXT PRIMARY KEY,
        name TEXT,
        image TEXT,
        pid INTEGER,
        state TEXT,
        started_at TEXT,
        rootfs_dir TEXT
    )`)

	if err != nil {
		return err
	}

	if _, err = s.db.Exec(`CREATE TABLE IF NOT EXISTS images (
        name TEXT PRIMARY KEY,
        path TEXT,
        created_at TEXT
    )`); err != nil {
		return err
	}

	rows, err := s.db.Query("PRAGMA table_info(containers)")
	if err != nil {
		return err
	}
	found := false
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dfltValue sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			rows.Close()
			return err
		}
		if name == "rootfs_dir" {
			found = true
			break
		}
	}
	rows.Close()
	if !found {
		if _, err := s.db.Exec("ALTER TABLE containers ADD COLUMN rootfs_dir TEXT"); err != nil {
			return err
		}
	}
	if _, err = s.db.Exec("PRAGMA journal_mode = WAL;"); err != nil {
		return err
	}
	if _, err = s.db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		return err
	}
	return nil
}

func (s *Store) SaveContainer(c ContainerInfo) error {
	_, err := s.db.Exec(`INSERT INTO containers(id, name, image, pid, state, started_at, rootfs_dir)
        VALUES (?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(id) DO UPDATE SET name=excluded.name,image=excluded.image,pid=excluded.pid,state=excluded.state,started_at=excluded.started_at,rootfs_dir=excluded.rootfs_dir`,
		c.ID, c.Name, c.Image, c.PID, c.State, c.StartedAt.Format(time.RFC3339), c.RootfsDir)
	return err
}

func (s *Store) ListContainers() ([]ContainerInfo, error) {
	rows, err := s.db.Query(`SELECT id, name, image, pid, state, started_at, rootfs_dir FROM containers`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ContainerInfo
	for rows.Next() {
		var c ContainerInfo
		var t, rootfsDir string
		if err := rows.Scan(&c.ID, &c.Name, &c.Image, &c.PID, &c.State, &t, &rootfsDir); err != nil {
			return nil, err
		}
		c.StartedAt, _ = time.Parse(time.RFC3339, t)
		c.RootfsDir = rootfsDir
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) GetContainer(id string) (ContainerInfo, error) {
	row := s.db.QueryRow(`SELECT id, name, image, pid, state, started_at, rootfs_dir FROM containers WHERE id = ?`, id)
	var c ContainerInfo
	var t, rootfsDir string
	if err := row.Scan(&c.ID, &c.Name, &c.Image, &c.PID, &c.State, &t, &rootfsDir); err != nil {
		return ContainerInfo{}, err
	}
	c.StartedAt, _ = time.Parse(time.RFC3339, t)
	c.RootfsDir = rootfsDir
	return c, nil
}

func (s *Store) DeleteContainer(id string) error {
	_, err := s.db.Exec(`DELETE FROM containers WHERE id = ?`, id)
	return err
}

func (s *Store) UpdateContainerState(id, state string) error {
	_, err := s.db.Exec(`UPDATE containers SET state = ? WHERE id = ?`, state, id)
	return err
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}
