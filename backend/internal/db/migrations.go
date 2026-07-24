// Package db provides access to Auto Applier's SQL migrations.
//
// Migrations are embedded so the binary is self-contained. Files follow the
// naming convention: NNNN_name.up.sql / NNNN_name.down.sql, where NNNN is a
// zero-padded, strictly increasing version.
package db

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Migration is a single up/down migration pair.
type Migration struct {
	Version int
	Name    string
	Up      string
	Down    string
}

// LoadMigrations reads and validates the embedded migrations, returning them
// sorted by ascending version. It errors on gaps, duplicate versions, missing
// up/down halves, or empty SQL.
func LoadMigrations() ([]Migration, error) {
	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return nil, fmt.Errorf("read migrations dir: %w", err)
	}

	type pair struct {
		name string
		up   string
		down string
	}
	byVersion := make(map[int]*pair)

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		version, base, direction, err := parseName(name)
		if err != nil {
			return nil, err
		}

		content, err := fs.ReadFile(migrationFiles, "migrations/"+name)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
		if strings.TrimSpace(string(content)) == "" {
			return nil, fmt.Errorf("migration %s is empty", name)
		}

		p := byVersion[version]
		if p == nil {
			p = &pair{name: base}
			byVersion[version] = p
		}
		if p.name != base {
			return nil, fmt.Errorf("version %d has conflicting names %q and %q", version, p.name, base)
		}
		switch direction {
		case "up":
			if p.up != "" {
				return nil, fmt.Errorf("duplicate up migration for version %d", version)
			}
			p.up = string(content)
		case "down":
			if p.down != "" {
				return nil, fmt.Errorf("duplicate down migration for version %d", version)
			}
			p.down = string(content)
		}
	}

	versions := make([]int, 0, len(byVersion))
	for v := range byVersion {
		versions = append(versions, v)
	}
	sort.Ints(versions)

	out := make([]Migration, 0, len(versions))
	for i, v := range versions {
		if v != i+1 {
			return nil, fmt.Errorf("migration versions must be contiguous from 1; got %d at position %d", v, i+1)
		}
		p := byVersion[v]
		if p.up == "" {
			return nil, fmt.Errorf("version %d missing up migration", v)
		}
		if p.down == "" {
			return nil, fmt.Errorf("version %d missing down migration", v)
		}
		out = append(out, Migration{Version: v, Name: p.name, Up: p.up, Down: p.down})
	}
	return out, nil
}

// parseName splits "0001_init.up.sql" into (1, "init", "up").
func parseName(name string) (version int, base, direction string, err error) {
	if !strings.HasSuffix(name, ".sql") {
		return 0, "", "", fmt.Errorf("migration %q is not a .sql file", name)
	}
	trimmed := strings.TrimSuffix(name, ".sql")

	dot := strings.LastIndex(trimmed, ".")
	if dot < 0 {
		return 0, "", "", fmt.Errorf("migration %q missing direction (.up/.down)", name)
	}
	direction = trimmed[dot+1:]
	if direction != "up" && direction != "down" {
		return 0, "", "", fmt.Errorf("migration %q has invalid direction %q", name, direction)
	}
	rest := trimmed[:dot]

	underscore := strings.Index(rest, "_")
	if underscore <= 0 {
		return 0, "", "", fmt.Errorf("migration %q missing version prefix", name)
	}
	version, err = strconv.Atoi(rest[:underscore])
	if err != nil {
		return 0, "", "", fmt.Errorf("migration %q has non-numeric version: %w", name, err)
	}
	base = rest[underscore+1:]
	if base == "" {
		return 0, "", "", fmt.Errorf("migration %q missing name", name)
	}
	return version, base, direction, nil
}
