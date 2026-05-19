package daemon

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"ksvm/pkg/api"
	"ksvm/pkg/kvm"
	"ksvm/pkg/web"
)

type Config struct {
	WebPort string
	MuxPort string
}

func Start(cfg Config) error {
	manager, err := kvm.NewManager()
	if err != nil {
		return err
	}
	defer manager.Close()

	// 1. Setup Web Server
	r := gin.Default()

	// API
	apiSvc := api.New(manager)
	apiSvc.Register(r)

	// Web UI
	r.GET("/", func(c *gin.Context) {
		c.FileFromFS("index.html", http.FS(web.Assets))
	})
	r.GET("/static/*filepath", func(c *gin.Context) {
		c.FileFromFS(c.Param("filepath"), http.FS(web.Assets))
	})

	webServer := &http.Server{
		Addr:    ":" + cfg.WebPort,
		Handler: r,
	}

	// 2. Setup Mux
	mux := NewMux(cfg.MuxPort, manager)

	// Start servers in goroutines
	go func() {
		fmt.Printf("Web UI listening on port %s\n", cfg.WebPort)
		if err := webServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Web server error: %v\n", err)
		}
	}()

	go func() {
		if err := mux.Start(); err != nil {
			fmt.Printf("Mux error: %v\n", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	fmt.Println("Shutting down daemon...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := webServer.Shutdown(ctx); err != nil {
		fmt.Printf("Web server shutdown error: %v\n", err)
	}

	fmt.Println("Daemon exited.")
	return nil
}
