package store

import (
	"database/sql"
	"time"

	_ "modernc.org/sqlite"
)

type ContainerInfo struct {
	ID             string
	Name           string
	Image          string
	PID            int
	State          string
	StartedAt      time.Time
	RootfsDir      string
	RestartCount   int
	HealthCmd      string
	HealthInterval int
	RestartMax     int
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
        rootfs_dir TEXT,
        restart_count INTEGER DEFAULT 0,
        health_cmd TEXT,
        health_interval INTEGER DEFAULT 0,
        restart_max INTEGER DEFAULT 0
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
	cols := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dfltValue sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			rows.Close()
			return err
		}
		cols[name] = true
	}
	rows.Close()
	if !cols["rootfs_dir"] {
		if _, err := s.db.Exec("ALTER TABLE containers ADD COLUMN rootfs_dir TEXT"); err != nil {
			return err
		}
	}
	if !cols["restart_count"] {
		if _, err := s.db.Exec("ALTER TABLE containers ADD COLUMN restart_count INTEGER DEFAULT 0"); err != nil {
			return err
		}
	}
	if !cols["health_cmd"] {
		if _, err := s.db.Exec("ALTER TABLE containers ADD COLUMN health_cmd TEXT"); err != nil {
			return err
		}
	}
	if !cols["health_interval"] {
		if _, err := s.db.Exec("ALTER TABLE containers ADD COLUMN health_interval INTEGER DEFAULT 0"); err != nil {
			return err
		}
	}
	if !cols["restart_max"] {
		if _, err := s.db.Exec("ALTER TABLE containers ADD COLUMN restart_max INTEGER DEFAULT 0"); err != nil {
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
	_, err := s.db.Exec(`INSERT INTO containers(id, name, image, pid, state, started_at, rootfs_dir, restart_count, health_cmd, health_interval, restart_max)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(id) DO UPDATE SET name=excluded.name,image=excluded.image,pid=excluded.pid,state=excluded.state,started_at=excluded.started_at,rootfs_dir=excluded.rootfs_dir,restart_count=excluded.restart_count,health_cmd=excluded.health_cmd,health_interval=excluded.health_interval,restart_max=excluded.restart_max`,
		c.ID, c.Name, c.Image, c.PID, c.State, c.StartedAt.Format(time.RFC3339), c.RootfsDir, c.RestartCount, c.HealthCmd, c.HealthInterval, c.RestartMax)
	return err
}

func (s *Store) ListContainers() ([]ContainerInfo, error) {
	rows, err := s.db.Query(`SELECT id, name, image, pid, state, started_at, rootfs_dir, restart_count, COALESCE(health_cmd, ''), health_interval, restart_max FROM containers`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ContainerInfo
	for rows.Next() {
		var c ContainerInfo
		var t, rootfsDir string
		if err := rows.Scan(&c.ID, &c.Name, &c.Image, &c.PID, &c.State, &t, &rootfsDir, &c.RestartCount, &c.HealthCmd, &c.HealthInterval, &c.RestartMax); err != nil {
			return nil, err
		}
		c.StartedAt, _ = time.Parse(time.RFC3339, t)
		c.RootfsDir = rootfsDir
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) GetContainer(id string) (ContainerInfo, error) {
	row := s.db.QueryRow(`SELECT id, name, image, pid, state, started_at, rootfs_dir, restart_count, COALESCE(health_cmd, ''), health_interval, restart_max FROM containers WHERE id = ?`, id)
	var c ContainerInfo
	var t, rootfsDir string
	if err := row.Scan(&c.ID, &c.Name, &c.Image, &c.PID, &c.State, &t, &rootfsDir, &c.RestartCount, &c.HealthCmd, &c.HealthInterval, &c.RestartMax); err != nil {
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

func (s *Store) UpdateContainerPID(id string, pid int) error {
	_, err := s.db.Exec(`UPDATE containers SET pid = ? WHERE id = ?`, pid, id)
	return err
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}
