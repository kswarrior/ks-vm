package daemon

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"ksvm/pkg/api"
	"ksvm/pkg/kvm"
	"ksvm/pkg/web"
)

type Config struct {
	WebPort    string
	MuxPort    string
	MasterUser string
	MasterPass string
}

func Start(cfg Config) error {
	manager, err := kvm.NewManager()
	if err != nil {
		return err
	}
	defer manager.Close()

	r := gin.Default()

	if cfg.MasterUser != "" {
		r.Use(gin.BasicAuth(gin.Accounts{
			cfg.MasterUser: cfg.MasterPass,
		}))
	}

	apiSvc := api.New(manager)
	apiSvc.Register(r)

	r.GET("/", func(c *gin.Context) {
		data, err := web.Assets.ReadFile("index.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "Internal Server Error")
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", data)
	})

	r.GET("/static/*filepath", func(c *gin.Context) {
		path := c.Param("filepath")[1:]
		data, err := web.Assets.ReadFile(path)
		if err != nil {
			c.String(http.StatusNotFound, "Not Found")
			return
		}

		contentType := "text/plain"
		if strings.HasSuffix(path, ".js") {
			contentType = "application/javascript"
		} else if strings.HasSuffix(path, ".css") {
			contentType = "text/css"
		} else if strings.HasSuffix(path, ".html") {
			contentType = "text/html"
		}
		c.Data(http.StatusOK, contentType, data)
	})

	webServer := &http.Server{
		Addr:    ":" + cfg.WebPort,
		Handler: r,
	}

	mux := NewMux(cfg.MuxPort, manager)

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
