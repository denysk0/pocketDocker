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
	Ports          string
	IpForwardOrig  string
	NetworkSetup   bool
	IPSuffix       int
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
	// Serialize migrations to avoid duplicate‑column races
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		}
	}()
	_, err = tx.Exec(`CREATE TABLE IF NOT EXISTS containers (
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
        restart_max INTEGER DEFAULT 0,
        ports TEXT
    )`)

	if err != nil {
		tx.Rollback()
		return err
	}

	if _, err = tx.Exec(`CREATE TABLE IF NOT EXISTS images (
        name TEXT PRIMARY KEY,
        path TEXT,
        created_at TEXT
    )`); err != nil {
		tx.Rollback()
		return err
	}

	rows, err := tx.Query("PRAGMA table_info(containers)")
	if err != nil {
		tx.Rollback()
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
			tx.Rollback()
			return err
		}
		cols[name] = true
	}
	rows.Close()
	if !cols["rootfs_dir"] {
		if _, err := tx.Exec("ALTER TABLE containers ADD COLUMN rootfs_dir TEXT"); err != nil {
			tx.Rollback()
			return err
		}
	}
	if !cols["restart_count"] {
		if _, err := tx.Exec("ALTER TABLE containers ADD COLUMN restart_count INTEGER DEFAULT 0"); err != nil {
			tx.Rollback()
			return err
		}
	}
	if !cols["health_cmd"] {
		if _, err := tx.Exec("ALTER TABLE containers ADD COLUMN health_cmd TEXT"); err != nil {
			tx.Rollback()
			return err
		}
	}
	if !cols["health_interval"] {
		if _, err := tx.Exec("ALTER TABLE containers ADD COLUMN health_interval INTEGER DEFAULT 0"); err != nil {
			tx.Rollback()
			return err
		}
	}
	if !cols["restart_max"] {
		if _, err := tx.Exec("ALTER TABLE containers ADD COLUMN restart_max INTEGER DEFAULT 0"); err != nil {
			tx.Rollback()
			return err
		}
	}
	if !cols["ports"] {
		if _, err := tx.Exec("ALTER TABLE containers ADD COLUMN ports TEXT"); err != nil {
			tx.Rollback()
			return err
		}
	}
	if !cols["ip_forward_orig"] {
		if _, err := tx.Exec("ALTER TABLE containers ADD COLUMN ip_forward_orig TEXT"); err != nil {
			tx.Rollback()
			return err
		}
	}
	if !cols["network_setup"] {
		if _, err := tx.Exec("ALTER TABLE containers ADD COLUMN network_setup INTEGER DEFAULT 0"); err != nil {
			tx.Rollback()
			return err
		}
	}
	if !cols["ip_suffix"] {
		if _, err := tx.Exec("ALTER TABLE containers ADD COLUMN ip_suffix INTEGER DEFAULT 0"); err != nil {
			tx.Rollback()
			return err
		}
	}
	if _, err = tx.Exec("PRAGMA journal_mode = WAL;"); err != nil {
		tx.Rollback()
		return err
	}
	if _, err = tx.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Store) SaveContainer(c ContainerInfo) error {
	_, err := s.db.Exec(`INSERT INTO containers(id, name, image, pid, state, started_at, rootfs_dir, restart_count, health_cmd, health_interval, restart_max, ports, ip_forward_orig, network_setup, ip_suffix)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(id) DO UPDATE SET name=excluded.name,image=excluded.image,pid=excluded.pid,state=excluded.state,started_at=excluded.started_at,rootfs_dir=excluded.rootfs_dir,restart_count=excluded.restart_count,health_cmd=excluded.health_cmd,health_interval=excluded.health_interval,restart_max=excluded.restart_max,ports=excluded.ports,ip_forward_orig=excluded.ip_forward_orig,network_setup=excluded.network_setup,ip_suffix=excluded.ip_suffix`,
		c.ID, c.Name, c.Image, c.PID, c.State, c.StartedAt.Format(time.RFC3339), c.RootfsDir, c.RestartCount, c.HealthCmd, c.HealthInterval, c.RestartMax, c.Ports, c.IpForwardOrig, c.NetworkSetup, c.IPSuffix)
	return err
}

func (s *Store) ListContainers() ([]ContainerInfo, error) {
	rows, err := s.db.Query(`SELECT id, name, image, pid, state, started_at, rootfs_dir, restart_count, COALESCE(health_cmd, ''), health_interval, restart_max, COALESCE(ports, ''), COALESCE(ip_forward_orig, ''), COALESCE(network_setup, 0), COALESCE(ip_suffix, 0) FROM containers`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ContainerInfo
	for rows.Next() {
		var c ContainerInfo
		var t, rootfsDir, ports, ipForwardOrig string
		var networkSetup int
		if err := rows.Scan(&c.ID, &c.Name, &c.Image, &c.PID, &c.State, &t, &rootfsDir, &c.RestartCount, &c.HealthCmd, &c.HealthInterval, &c.RestartMax, &ports, &ipForwardOrig, &networkSetup, &c.IPSuffix); err != nil {
			return nil, err
		}
		c.StartedAt, _ = time.Parse(time.RFC3339, t)
		c.RootfsDir = rootfsDir
		c.Ports = ports
		c.IpForwardOrig = ipForwardOrig
		c.NetworkSetup = networkSetup != 0
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) GetContainer(id string) (ContainerInfo, error) {
	row := s.db.QueryRow(`SELECT id, name, image, pid, state, started_at, rootfs_dir, restart_count, COALESCE(health_cmd, ''), health_interval, restart_max, COALESCE(ports, ''), COALESCE(ip_forward_orig, ''), COALESCE(network_setup, 0), COALESCE(ip_suffix, 0) FROM containers WHERE id = ?`, id)
	var c ContainerInfo
	var t, rootfsDir, ports, ipForwardOrig string
	var networkSetup int
	if err := row.Scan(&c.ID, &c.Name, &c.Image, &c.PID, &c.State, &t, &rootfsDir, &c.RestartCount, &c.HealthCmd, &c.HealthInterval, &c.RestartMax, &ports, &ipForwardOrig, &networkSetup, &c.IPSuffix); err != nil {
		return ContainerInfo{}, err
	}
	c.StartedAt, _ = time.Parse(time.RFC3339, t)
	c.RootfsDir = rootfsDir
	c.Ports = ports
	c.IpForwardOrig = ipForwardOrig
	c.NetworkSetup = networkSetup != 0
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
