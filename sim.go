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
	ExecutePerTime = 20
	MaxLifeTime    = 60
	MaxIdleTime    = 60
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
	ctx := context.Background()
	// mysql>  SET GLOBAL max_connections = 50; => "Error 1040: Too many connections
	simTest(ctx, 100, 51, 51)

	//  maxIdle=-1代表不使用连接池
	// 连接复用的前提是，当前打开连接数 <= 最大连接数
	// openConn = Idle + InUse

	// 无连接池 => MaxIdleClosed==MaxOpenConnections*ExecutePerTime，WaitCount == 0. 用完就关闭
	//simTest(ctx, 30, 30, -1)
	//simTest(ctx, 20, 30, -1)

	// 无连接池 => MaxIdleClosed > 0. WaitCount > 0，因为并发数大于最大连接数，存在排队获取连接，释放连接时也可以复用，所以MaxIdleClosed!=MaxOpenConnections*ExecutePerTime
	//simTest(ctx, 30, 20, -1)

	// 有连接池 => MaxIdleClosed=WaitCount=0
	//simTest(ctx, 30, 30, 30)
	//simTest(ctx, 20, 30, 30)

	// 有连接池 => MaxIdleClosed=0. WaitCount > 0 。并发数大于最大连接数，存在排队，连接复用时，优先返回给排队队列，其次放回连接池
	//simTest(ctx, 30, 20, 20)

	// 有连接池 => MaxIdleClosed > 0 . WaitCount > 0， 连接池太小，导致连接没法复用
	//simTest(ctx, 30, 20, 1)
}

func simTest(ctx context.Context, concurrencyConn, maxOpen, maxIdle int) {
	var (
		db  *sql.DB
		wg  sync.WaitGroup
		err error
		t1  time.Time
	)
	if db, err = GetDB(); err != nil {
		logger.Fatal("exception: %v", zap.Error(err))
		return
	}
	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxIdle)
	db.SetConnMaxLifetime(MaxLifeTime * time.Second)
	db.SetConnMaxIdleTime(MaxIdleTime * time.Second)

	wg = sync.WaitGroup{}
	wg.Add(concurrencyConn + 1)
	t1 = time.Now()
	for i := 0; i < concurrencyConn; i++ {
		go func() {
			for i := 0; i < ExecutePerTime; i++ {
				//_, _ = db.ExecContext(ctx, "SELECT SLEEP(0.5)")
				_, err = db.ExecContext(ctx, "SELECT 10")
				if err != nil {
					logger.Error("DBExecErr", zap.Error(err))
				}
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
				zap.Int64("MaxIdleTimeClosed", int64(db.Stats().MaxIdleTimeClosed)),
				zap.Int64("MaxLifetimeClosed", int64(db.Stats().MaxLifetimeClosed)),
			)
			if db.Stats().InUse == 0 && db.Stats().MaxIdleClosed >= 0 {
				break
			}
			time.Sleep(100 * time.Microsecond)
		}
		wg.Done()
	}()
	wg.Wait()
	logger.Info("Timing", zap.Int64("Cost", time.Now().Sub(t1).Milliseconds()))
}
