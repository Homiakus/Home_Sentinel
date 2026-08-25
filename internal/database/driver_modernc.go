//go:build !sqlite_cgo

package database

import _ "modernc.org/sqlite"

const driverName = "sqlite"
