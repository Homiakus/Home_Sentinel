//go:build sqlite_cgo

package database

import _ "github.com/Homiakus/Home_Sentinel/internal/database/sqlitecgo"

const driverName = "sentinel-sqlite3-cgo"
