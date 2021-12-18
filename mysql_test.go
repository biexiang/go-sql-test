package main

import (
	"context"
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"testing"
)

//  docker pull mysql:5.7
//  docker run -itd --name mysql -p 3306:3306 -e MYSQL_ROOT_PASSWORD=123456 mysql:5.7
//  go test -bench .
func BenchmarkConnectMySQL(b *testing.B) {
	fnConnectMySQL := func(dsn string) (*sql.DB, error) {
		return sql.Open("mysql", dsn)
	}
	db, err := fnConnectMySQL("root:123456@tcp(127.0.0.1:3306)/test")
	if err != nil {
		b.Fatalf("mysql connect error : %s", err.Error())
		return
	}
	ctx := context.Background()
	db.SetMaxIdleConns(-1)
	b.ResetTimer()
	b.Run("withoutConnPool", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = db.ExecContext(ctx, "SELECT SLEEP(0.5)")
		}
	})
	db.SetMaxIdleConns(1)
	b.ResetTimer()
	b.Run("withConnPool", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = db.ExecContext(ctx, "SELECT SLEEP(0.5)")
		}
	})
}
