//go:build sqlite_cgo

package sqlitecgo

/*
#cgo LDFLAGS: -lsqlite3
#include <sqlite3.h>
#include <stdlib.h>
*/
import "C"

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"
)

const driverName = "sentinel-sqlite3-cgo"

func init() { sql.Register(driverName, &drv{}) }

type drv struct{}

func (d *drv) Open(name string) (driver.Conn, error) {
	path, pragmas, err := parseDSN(name)
	if err != nil {
		return nil, err
	}
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	var db *C.sqlite3
	flags := C.int(C.SQLITE_OPEN_READWRITE | C.SQLITE_OPEN_CREATE | C.SQLITE_OPEN_FULLMUTEX | C.SQLITE_OPEN_URI)
	if rc := C.sqlite3_open_v2(cpath, &db, flags, nil); rc != C.SQLITE_OK {
		msg := "sqlite open failed"
		if db != nil {
			msg = C.GoString(C.sqlite3_errmsg(db))
			C.sqlite3_close_v2(db)
		}
		return nil, errors.New(msg)
	}
	conn := &conn{db: db}
	for _, p := range pragmas {
		if _, err := conn.exec(context.Background(), "PRAGMA "+p, nil); err != nil {
			_ = conn.Close()
			return nil, err
		}
	}
	return conn, nil
}

func parseDSN(name string) (string, []string, error) {
	if !strings.HasPrefix(name, "file:") {
		return name, nil, nil
	}
	u, err := url.Parse(name)
	if err != nil {
		return "", nil, err
	}
	path := u.Path
	if u.Opaque != "" {
		path = u.Opaque
	}
	if path == "sentinel-memory" || u.Query().Get("mode") == "memory" {
		path = "file:sentinel-memory?mode=memory&cache=shared"
	}
	return path, u.Query()["_pragma"], nil
}

type conn struct {
	mu   sync.Mutex
	db   *C.sqlite3
	inTx bool
}

func (c *conn) Prepare(query string) (driver.Stmt, error) {
	return c.PrepareContext(context.Background(), query)
}
func (c *conn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.db == nil {
		return nil, driver.ErrBadConn
	}
	st, err := prepare(c.db, query)
	if err != nil {
		return nil, err
	}
	return &stmt{c: c, st: st}, nil
}
func (c *conn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.db == nil {
		return nil
	}
	rc := C.sqlite3_close_v2(c.db)
	if rc != C.SQLITE_OK {
		return fmt.Errorf("sqlite close: %s", C.GoString(C.sqlite3_errmsg(c.db)))
	}
	c.db = nil
	return nil
}
func (c *conn) Begin() (driver.Tx, error) { return c.BeginTx(context.Background(), driver.TxOptions{}) }
func (c *conn) BeginTx(ctx context.Context, _ driver.TxOptions) (driver.Tx, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.inTx {
		return nil, errors.New("transaction already active")
	}
	if _, err := c.execLocked(ctx, "BEGIN", nil); err != nil {
		return nil, err
	}
	c.inTx = true
	return &tx{c: c}, nil
}
func (c *conn) Ping(ctx context.Context) error {
	_, err := c.ExecContext(ctx, "SELECT 1", nil)
	return err
}
func (c *conn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	vals := namedToValues(args)
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.execLocked(ctx, query, vals)
}
func (c *conn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	vals := namedToValues(args)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.db == nil {
		return nil, driver.ErrBadConn
	}
	st, err := prepare(c.db, query)
	if err != nil {
		return nil, err
	}
	if err := bindAll(st, vals); err != nil {
		C.sqlite3_finalize(st)
		return nil, err
	}
	cols := C.sqlite3_column_count(st)
	names := make([]string, int(cols))
	for i := range names {
		names[i] = C.GoString(C.sqlite3_column_name(st, C.int(i)))
	}
	return &rows{c: c, st: st, cols: names}, nil
}
func (c *conn) exec(ctx context.Context, q string, vals []driver.Value) (driver.Result, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.execLocked(ctx, q, vals)
}
func (c *conn) execLocked(ctx context.Context, query string, vals []driver.Value) (driver.Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if c.db == nil {
		return nil, driver.ErrBadConn
	}
	// sqlite3_prepare_v2 consumes one statement at a time. Migrations may contain
	// multiple statements, so loop over the tail while binding only the first
	// statement (normal parameterized Exec calls are single-statement).
	remaining := query
	first := true
	var lastID, affected int64
	for strings.TrimSpace(remaining) != "" {
		st, tail, err := prepareWithTail(c.db, remaining)
		if err != nil {
			return nil, err
		}
		if st == nil {
			remaining = tail
			continue
		}
		if first {
			if err := bindAll(st, vals); err != nil {
				C.sqlite3_finalize(st)
				return nil, err
			}
			first = false
		}
		for {
			if err := ctx.Err(); err != nil {
				C.sqlite3_finalize(st)
				return nil, err
			}
			rc := C.sqlite3_step(st)
			if rc == C.SQLITE_ROW {
				continue
			}
			if rc != C.SQLITE_DONE {
				msg := C.GoString(C.sqlite3_errmsg(c.db))
				C.sqlite3_finalize(st)
				return nil, errors.New(msg)
			}
			break
		}
		C.sqlite3_finalize(st)
		lastID = int64(C.sqlite3_last_insert_rowid(c.db))
		affected += int64(C.sqlite3_changes(c.db))
		remaining = tail
	}
	return result{id: lastID, rows: affected}, nil
}

type tx struct{ c *conn }

func (t *tx) Commit() error   { return t.finish("COMMIT") }
func (t *tx) Rollback() error { return t.finish("ROLLBACK") }
func (t *tx) finish(q string) error {
	t.c.mu.Lock()
	defer t.c.mu.Unlock()
	if !t.c.inTx {
		return errors.New("transaction not active")
	}
	_, err := t.c.execLocked(context.Background(), q, nil)
	if err == nil {
		t.c.inTx = false
	}
	return err
}

type stmt struct {
	c  *conn
	st *C.sqlite3_stmt
}

func (s *stmt) Close() error {
	s.c.mu.Lock()
	defer s.c.mu.Unlock()
	if s.st != nil {
		C.sqlite3_finalize(s.st)
		s.st = nil
	}
	return nil
}
func (s *stmt) NumInput() int {
	if s.st == nil {
		return -1
	}
	return int(C.sqlite3_bind_parameter_count(s.st))
}
func (s *stmt) Exec(args []driver.Value) (driver.Result, error) {
	s.c.mu.Lock()
	defer s.c.mu.Unlock()
	return execPrepared(s.c.db, s.st, args)
}
func (s *stmt) Query(args []driver.Value) (driver.Rows, error) {
	s.c.mu.Lock()
	defer s.c.mu.Unlock()
	if err := bindAll(s.st, args); err != nil {
		return nil, err
	}
	n := int(C.sqlite3_column_count(s.st))
	names := make([]string, n)
	for i := range names {
		names[i] = C.GoString(C.sqlite3_column_name(s.st, C.int(i)))
	}
	return &rows{c: s.c, st: s.st, cols: names, ownedByStmt: true}, nil
}

type rows struct {
	c           *conn
	st          *C.sqlite3_stmt
	cols        []string
	done        bool
	ownedByStmt bool
}

func (r *rows) Columns() []string { return append([]string(nil), r.cols...) }
func (r *rows) Close() error {
	r.c.mu.Lock()
	defer r.c.mu.Unlock()
	if r.st != nil && !r.ownedByStmt {
		C.sqlite3_finalize(r.st)
	}
	r.st = nil
	return nil
}
func (r *rows) Next(dest []driver.Value) error {
	r.c.mu.Lock()
	defer r.c.mu.Unlock()
	if r.done || r.st == nil {
		return io.EOF
	}
	rc := C.sqlite3_step(r.st)
	if rc == C.SQLITE_DONE {
		r.done = true
		return io.EOF
	}
	if rc != C.SQLITE_ROW {
		return errors.New(C.GoString(C.sqlite3_errmsg(r.c.db)))
	}
	for i := range dest {
		dest[i] = columnValue(r.st, C.int(i))
	}
	return nil
}

type result struct{ id, rows int64 }

func (r result) LastInsertId() (int64, error) { return r.id, nil }
func (r result) RowsAffected() (int64, error) { return r.rows, nil }

func namedToValues(args []driver.NamedValue) []driver.Value {
	out := make([]driver.Value, len(args))
	for i, a := range args {
		out[i] = a.Value
	}
	return out
}
func prepare(db *C.sqlite3, q string) (*C.sqlite3_stmt, error) {
	st, _, err := prepareWithTail(db, q)
	return st, err
}
func prepareWithTail(db *C.sqlite3, q string) (*C.sqlite3_stmt, string, error) {
	cq := C.CString(q)
	defer C.free(unsafe.Pointer(cq))
	var st *C.sqlite3_stmt
	var tail *C.char
	if rc := C.sqlite3_prepare_v2(db, cq, -1, &st, &tail); rc != C.SQLITE_OK {
		return nil, "", errors.New(C.GoString(C.sqlite3_errmsg(db)))
	}
	rem := ""
	if tail != nil {
		rem = C.GoString(tail)
	}
	return st, rem, nil
}
func bindAll(st *C.sqlite3_stmt, args []driver.Value) error {
	C.sqlite3_clear_bindings(st)
	C.sqlite3_reset(st)
	for i, v := range args {
		if err := bind(st, C.int(i+1), v); err != nil {
			return err
		}
	}
	return nil
}
func bind(st *C.sqlite3_stmt, i C.int, v driver.Value) error {
	var rc C.int
	switch x := v.(type) {
	case nil:
		rc = C.sqlite3_bind_null(st, i)
	case int64:
		rc = C.sqlite3_bind_int64(st, i, C.sqlite3_int64(x))
	case float64:
		rc = C.sqlite3_bind_double(st, i, C.double(x))
	case bool:
		if x {
			rc = C.sqlite3_bind_int64(st, i, 1)
		} else {
			rc = C.sqlite3_bind_int64(st, i, 0)
		}
	case []byte:
		if len(x) == 0 {
			rc = C.sqlite3_bind_blob(st, i, nil, 0, C.SQLITE_TRANSIENT)
		} else {
			rc = C.sqlite3_bind_blob(st, i, unsafe.Pointer(&x[0]), C.int(len(x)), C.SQLITE_TRANSIENT)
		}
	case string:
		cs := C.CString(x)
		rc = C.sqlite3_bind_text(st, i, cs, C.int(len(x)), C.SQLITE_TRANSIENT)
		C.free(unsafe.Pointer(cs))
	case time.Time:
		s := x.UTC().Format(time.RFC3339Nano)
		cs := C.CString(s)
		rc = C.sqlite3_bind_text(st, i, cs, C.int(len(s)), C.SQLITE_TRANSIENT)
		C.free(unsafe.Pointer(cs))
	default:
		return fmt.Errorf("unsupported sqlite bind type %T", v)
	}
	if rc != C.SQLITE_OK {
		return fmt.Errorf("sqlite bind failed: %d", int(rc))
	}
	return nil
}
func execPrepared(db *C.sqlite3, st *C.sqlite3_stmt, args []driver.Value) (driver.Result, error) {
	if err := bindAll(st, args); err != nil {
		return nil, err
	}
	for {
		rc := C.sqlite3_step(st)
		if rc == C.SQLITE_ROW {
			continue
		}
		if rc != C.SQLITE_DONE {
			return nil, errors.New(C.GoString(C.sqlite3_errmsg(db)))
		}
		break
	}
	return result{id: int64(C.sqlite3_last_insert_rowid(db)), rows: int64(C.sqlite3_changes(db))}, nil
}
func columnValue(st *C.sqlite3_stmt, i C.int) driver.Value {
	switch C.sqlite3_column_type(st, i) {
	case C.SQLITE_INTEGER:
		return int64(C.sqlite3_column_int64(st, i))
	case C.SQLITE_FLOAT:
		return float64(C.sqlite3_column_double(st, i))
	case C.SQLITE_TEXT:
		p := C.sqlite3_column_text(st, i)
		n := C.sqlite3_column_bytes(st, i)
		return C.GoStringN((*C.char)(unsafe.Pointer(p)), n)
	case C.SQLITE_BLOB:
		p := C.sqlite3_column_blob(st, i)
		n := C.sqlite3_column_bytes(st, i)
		if p == nil || n == 0 {
			return []byte{}
		}
		return C.GoBytes(p, n)
	default:
		return nil
	}
}

var _ driver.Driver = (*drv)(nil)
var _ driver.Conn = (*conn)(nil)
var _ driver.ConnBeginTx = (*conn)(nil)
var _ driver.ConnPrepareContext = (*conn)(nil)
var _ driver.ExecerContext = (*conn)(nil)
var _ driver.QueryerContext = (*conn)(nil)
var _ driver.Pinger = (*conn)(nil)

// Keep strconv referenced in older toolchains where URL normalization differs.
var _ = strconv.IntSize
