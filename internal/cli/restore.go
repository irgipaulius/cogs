package cli

import (
	"context"
	"database/sql"
	"flag"
	"fmt"

	_ "modernc.org/sqlite"
)

func cmdRestoreTest(args []string) error {
	fs := flag.NewFlagSet("restore-test", flag.ExitOnError)
	_ = fs.Parse(args)
	rest := fs.Args()
	if len(rest) < 1 {
		return fmt.Errorf("usage: cogs restore-test <backup.db>")
	}
	path := rest[0]
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=busy_timeout(5000)")
	if err != nil {
		return err
	}
	defer db.Close()
	var result string
	if err := db.QueryRowContext(context.Background(), "PRAGMA integrity_check").Scan(&result); err != nil {
		return err
	}
	if result != "ok" {
		return fmt.Errorf("integrity_check: %s", result)
	}
	fmt.Println("integrity_check ok")
	return nil
}
