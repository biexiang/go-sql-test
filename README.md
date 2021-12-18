### Go database/sql 源码阅读
> 基于 release-branch.go1.17
* Benchmark 连接池测试
* 简单介绍 database/sql 库，包括结构体和主要的方法
* 介绍主要函数的调用逻辑
* 用OneNote看源码：[Link](https://1drv.ms/u/s!Aj5EXSoGCuo4eBZb0BjbPiAMJGQ)
* 介绍最近几个版本的commit changes
* 结合实际使用
* Q&A？？？ 极端情况的case和解决办法 https://juejin.cn/post/6844903924017659911

#### 源码结构
```
└─database
    └─sql
        │  convert.go           // 数据类型转换工具
        │  ctxutil.go           // SQL查询接口的一层wrapper，主要处理context close的问题
        │  sql.go               // 数据库驱动和SQL的通用接口和类型，包括连接池、连接、事务、statement等
        │
        └─driver
                driver.go       // 定义了很多的接口，需要具体数据库驱动实现
                types.go        // 数据类型的别名和转换
```

#### DB结构
```
type DB struct {
    		
  waitDuration int64 							// 等待新的连接所需要的总时间，原子操作，waitDuration放在第一个是为了避免32位系统使用64位原子操作的坑，https://toutiao.io/posts/jagmqm/preview

  connector driver.Connector 					// 由数据库驱动实现的connector
  numClosed uint64           					// 关闭的连接数

  mu           sync.Mutex 						// protects following fields
  freeConn     []*driverConn                	// 连接池
  connRequests map[uint64]chan connRequest  	// 阻塞等待连接的随机队列
  nextRequest  uint64                       	// 下一个连接的key
  numOpen      int                          	// 打开的和正在打开的连接数
    
  openerCh          chan struct{}  				// channel用于通知建立新连接
  closed            bool           				// 当前数据库是否关闭
  dep               map[finalCloser]depSet		
  lastPut           map[*driverConn]string		// debug only
  maxIdleCount      int                    		// 连接池大小，0代表默认连接数2，负数代表0
  maxOpen           int                    		// 最大打开的连接数，包含在连接池中的闲散连接，小于等于0表示不限制
  maxLifetime       time.Duration          		// 一个连接在连接池中最长的生存时间
  maxIdleTime       time.Duration          		// Go1.5添加，一个连接在被关掉前的最大空闲时间
    
  cleanerCh         chan struct{}				// channel用于通知清理连接
  waitCount         int64          				// 等待的连接数，如果maxIdelCount为0，waitCount就是一直为0
  maxIdleClosed     int64          				// Go1.5添加，释放连接时，因为连接池已满而被关闭的连接数
  maxLifetimeClosed int64          				// 连接超过生存时间后而被关闭的连接数

  stop func() 									// stop cancels the connection opener and the session resetter.
}

```

#### DriverConn结构
```
type driverConn struct {
  db        *DB							// DB指针
  createdAt time.Time					// 创建时间

  sync.Mutex  							// guards following
  ci          driver.Conn				// 建立的连接
  needReset   bool 						// The connection session should be reset before use if true.
  closed      bool 						// 确定连接最终都关闭后才是最终关闭
  finalClosed bool 						// 确定依赖都被关闭后，才会执行最后的关闭
  openStmt    map[*driverStmt]bool

  // guarded by db.mu
  inUse      bool
  returnedAt time.Time 					// 类似于updated_time，For maxIdleTime，计算空闲时间
  onPut      []func() 					// code (with db.mu held) run when conn is next returned
  dbmuClosed bool     					// same as closed, but guarded by db.mu, for removeClosedStmtLocked
}
```
