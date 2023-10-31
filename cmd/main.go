package main

import (
	"context"
	"flag"
	"fmt"
	"math"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/gin-gonic/contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/node-real/go-pkg/log"
	middlewares "github.com/node-real/go-pkg/middlewares/gin"
	"github.com/node-real/go-pkg/utils"

	"github.com/reneecok/puissant-proxy/node"
	"github.com/reneecok/puissant-proxy/proxy"
)

const serviceName = "puissant-proxy"

var configPath = flag.String("config",
	"./configs/config.toml",
	"puissant proxy config file path")

func init() {
	gin.SetMode(gin.ReleaseMode)

	// the best factor concluded by load testing.
	maxProcs := int(math.Ceil(1.5 * float64(runtime.GOMAXPROCS(0))))
	runtime.GOMAXPROCS(maxProcs)
	fmt.Println("set GOMAXPROCS to", runtime.GOMAXPROCS(0))
}

func main() {
	defer log.Stop()

	flag.Parse()

	cfg := loadConfig(*configPath)
	initLogger(&cfg.Log)

	utils.OpenPrometheusAndPprof(cfg.Debug.ListenAddr)

	ctx := createLaunchContext()

	log.Infow("puissant proxy start", "configPath", *configPath, "config", cfg)

	nodes := node.NewNodes(ctx, &cfg.Node)

	rpcServer := rpc.NewServer()
	service := proxy.NewValidatorProxy(&cfg.Proxy, nodes)
	if err := rpcServer.RegisterName("eth", service); err != nil {
		panic(err)
	}

	app := gin.New()
	app.Use(
		middlewares.ConcurrencyLimiter(cfg.Proxy.Concurrency),
		middlewares.PanicRecovery(),
		gzip.Gzip(gzip.DefaultCompression),
	)

	app.POST("/", gin.WrapH(rpcServer))

	if err := app.Run(cfg.Proxy.HTTPListenAddr); err != nil {
		log.Errorf("fail to run rpc server, err:%v", err)
	}
}

func initLogger(cfg *LogConfig) {
	lvl, _ := log.ParseLevel(cfg.Level)
	log.Init(lvl, log.StandardizePath(cfg.RootDir, serviceName))
}

func createLaunchContext() context.Context {
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	ctx, cancelCtx := context.WithCancel(context.Background())

	go func() {
		defer close(shutdown)
		sig := <-shutdown
		log.Infof("received signal:%v, gracefully shutdown", sig.Signal)
		cancelCtx()
	}()

	return ctx
}
