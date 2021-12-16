package main

import (
	"context"
	"database/sql"
	"encoding/json"
	_ "github.com/go-sql-driver/mysql"
	"go.uber.org/zap"
	"sync"
	"time"
)

const (
	DSN            = "root:123456@tcp(127.0.0.1:3306)/test"
	Concurrency    = 100
	ExecutePerTime = 100
)

var logger *zap.Logger

func init() {
	var (
		cfg zap.Config
		err error
	)
	rawJSON := []byte(`{
    "level":"debug",
    "encoding":"json",
    "outputPaths": ["stdout", "std.log"],
    "errorOutputPaths": ["stderr"],
    "initialFields":{"name":"dj"},
    "encoderConfig": {
      "messageKey": "message",
      "levelKey": "level",
      "levelEncoder": "lowercase"
    }
  }`)
	if err = json.Unmarshal(rawJSON, &cfg); err != nil {
		panic(err)
	}
	logger, err = cfg.Build()
	if err != nil {
		panic(err)
	}
	defer func() {
		_ = logger.Sync()
	}()
}

func GetDB() (*sql.DB, error) {
	return sql.Open("mysql", DSN)
}

func main() {
	var (
		ctx context.Context
		db  *sql.DB
		wg  sync.WaitGroup
		err error
	)
	if db, err = GetDB(); err != nil {
		logger.Fatal("exception: %v", zap.Error(err))
		return
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(20)
	db.SetConnMaxLifetime(180)
	db.SetConnMaxIdleTime(60)

	ctx = context.Background()
	wg = sync.WaitGroup{}
	wg.Add(Concurrency + 1)
	for i := 0; i < Concurrency; i++ {
		go func() {
			for i := 0; i < ExecutePerTime; i++ {
				_, _ = db.ExecContext(ctx, "SELECT SLEEP(0.5)")
				//_, _ = db.ExecContext(ctx, "SELECT 10")
			}
			wg.Done()
		}()
	}
	go func() {
		for {
			logger.Info("Stat",
				zap.Int("MaxOpenConnections", db.Stats().MaxOpenConnections),
				zap.Int("OpenConn", db.Stats().OpenConnections),
				zap.Int("Idle", db.Stats().Idle),
				zap.Int("InUse", db.Stats().InUse),
				zap.Int64("MaxIdleClosed", db.Stats().MaxIdleClosed),
				zap.Int64("WaitCount", db.Stats().WaitCount),
				zap.Int64("WaitDuration", int64(db.Stats().WaitDuration)),
			)
			if db.Stats().InUse == 0 && db.Stats().MaxIdleClosed >= 0 {
				break
			}
			time.Sleep(100 * time.Microsecond)
		}
		wg.Done()
	}()
	wg.Wait()
}
