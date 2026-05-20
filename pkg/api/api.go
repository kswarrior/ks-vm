package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"ksvm/pkg/kvm"
)

type User struct {
	Username    string   `json:"username"`
	Email       string   `json:"email"`
	Password    string   `json:"password,omitempty"`
	Permissions []string `json:"permissions"`
}

type LogEntry struct {
	ID        int    `json:"id"`
	Timestamp string `json:"timestamp"`
	User      string `json:"user"`
	IP        string `json:"ip"`
	Action    string `json:"action"`
	Target    string `json:"target"`
}

type API struct {
	manager *kvm.Manager
	users   []User
	uMu     sync.RWMutex
	logs    []LogEntry
	lMu     sync.RWMutex
}

func New(m *kvm.Manager) *API {
	api := &API{manager: m}
	api.loadUsers()
	return api
}

func (a *API) loadUsers() {
	path := filepath.Join(kvm.BaseDir, "users.json")
	data, err := os.ReadFile(path)
	if err == nil {
		json.Unmarshal(data, &a.users)
	}
	if len(a.users) == 0 {
		a.users = []User{{Username: "admin", Email: "admin@ksvm.local", Permissions: []string{"owner"}}}
	}
}

func (a *API) saveUsers() {
	path := filepath.Join(kvm.BaseDir, "users.json")
	data, _ := json.Marshal(a.users)
	os.WriteFile(path, data, 0644)
}

func (a *API) Register(r *gin.Engine) {
	v1 := r.Group("/api/v1")
	{
		v1.GET("/instances", a.listInstances)
		v1.GET("/images", a.listImages)
		v1.GET("/info/:name", a.getInstanceInfo)

		// Lifecycle Actions
		v1.POST("/deploy", a.deployInstance)
		v1.POST("/launch/:name", a.launchInstance)
		v1.POST("/stop/:name", a.stopInstance)
		v1.POST("/restart/:name", a.restartInstance)
		v1.POST("/suspend/:name", a.suspendInstance)
		v1.POST("/resume/:name", a.resumeInstance)
		v1.PUT("/update/:name", a.updateInstance)
		v1.DELETE("/delete/:name", a.deleteInstance)

		// System & Users
		v1.GET("/monitor", a.getMonitorData)
		v1.GET("/users", a.listUsers)
		v1.POST("/users", a.createUser)
		v1.GET("/logs", a.getAuditLogs)
		v1.POST("/logs/undo/:id", a.undoAction)
	}
}

func (a *API) addLog(c *gin.Context, action, target string) {
	a.lMu.Lock()
	defer a.lMu.Unlock()
	entry := LogEntry{
		ID:        len(a.logs),
		Timestamp: time.Now().Format("2006-01-02 15:04:05"),
		User:      "admin", // Placeholder
		IP:        c.ClientIP(),
		Action:    action,
		Target:    target,
	}
	a.logs = append([]LogEntry{entry}, a.logs...)
}

func (a *API) listInstances(c *gin.Context) {
	instances, err := a.manager.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, instances)
}

func (a *API) listImages(c *gin.Context) {
	images, err := a.manager.ListImages()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, images)
}

func (a *API) getInstanceInfo(c *gin.Context) {
	name := c.Param("name")
	info, err := a.manager.Info(name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, info)
}

func (a *API) deployInstance(c *gin.Context) {
	var req struct {
		Name     string `json:"name" binding:"required"`
		Image    string `json:"image" binding:"required"`
		User     string `json:"user"`
		Password string `json:"password"`
		CPUs     uint   `json:"cpus"`
		MemoryMB uint   `json:"memory_mb"`
		DiskGB   uint   `json:"disk_gb"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	opts := kvm.DeployOptions{
		User:     req.User,
		Password: req.Password,
		CPUs:     req.CPUs,
		MemoryMB: req.MemoryMB,
		DiskGB:   req.DiskGB,
	}
	if err := a.manager.Deploy(req.Name, req.Image, opts); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	a.addLog(c, "deploy", req.Name)
	c.JSON(http.StatusCreated, gin.H{"message": "Deployed " + req.Name})
}

func (a *API) launchInstance(c *gin.Context) {
	name := c.Param("name")
	if err := a.manager.Launch(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	a.addLog(c, "launch", name)
	c.JSON(http.StatusOK, gin.H{"message": "Started " + name})
}

func (a *API) stopInstance(c *gin.Context) {
	name := c.Param("name")
	if err := a.manager.Stop(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	a.addLog(c, "stop", name)
	c.JSON(http.StatusOK, gin.H{"message": "Stopped " + name})
}

func (a *API) restartInstance(c *gin.Context) {
	name := c.Param("name")
	if err := a.manager.Restart(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	a.addLog(c, "restart", name)
	c.JSON(http.StatusOK, gin.H{"message": "Restarted " + name})
}

func (a *API) suspendInstance(c *gin.Context) {
	name := c.Param("name")
	if err := a.manager.Suspend(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	a.addLog(c, "suspend", name)
	c.JSON(http.StatusOK, gin.H{"message": "Suspended " + name})
}

func (a *API) resumeInstance(c *gin.Context) {
	name := c.Param("name")
	if err := a.manager.Resume(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	a.addLog(c, "resume", name)
	c.JSON(http.StatusOK, gin.H{"message": "Resumed " + name})
}

func (a *API) updateInstance(c *gin.Context) {
	name := c.Param("name")
	var req struct {
		MemoryMB uint `json:"memory_mb"`
		CPUs     uint `json:"cpus"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := a.manager.Update(name, req.MemoryMB, req.CPUs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	a.addLog(c, "update", name)
	c.JSON(http.StatusOK, gin.H{"message": "Updated " + name})
}

func (a *API) deleteInstance(c *gin.Context) {
	name := c.Param("name")
	if err := a.manager.Delete(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	a.addLog(c, "delete", name)
	c.JSON(http.StatusOK, gin.H{"message": "Deleted " + name})
}

func (a *API) getMonitorData(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"cpu": 12.5,
		"ram": 8.2,
		"active_instances": 3,
	})
}

func (a *API) listUsers(c *gin.Context) {
	a.uMu.RLock()
	defer a.uMu.RUnlock()
	c.JSON(http.StatusOK, a.users)
}

func (a *API) createUser(c *gin.Context) {
	var user User
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	a.uMu.Lock()
	defer a.uMu.Unlock()
	a.users = append(a.users, user)
	a.saveUsers()
	c.JSON(http.StatusCreated, user)
}

func (a *API) getAuditLogs(c *gin.Context) {
	a.lMu.RLock()
	defer a.lMu.RUnlock()
	c.JSON(http.StatusOK, a.logs)
}

func (a *API) undoAction(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	a.lMu.RLock()
	var log LogEntry
	for _, l := range a.logs {
		if l.ID == id {
			log = l
			break
		}
	}
	a.lMu.RUnlock()

	if log.Target != "" {
		switch log.Action {
		case "stop":
			a.manager.Launch(log.Target)
		case "launch":
			a.manager.Stop(log.Target)
		case "deploy":
			a.manager.Delete(log.Target)
		}
		a.addLog(c, "undo", log.Action+" "+log.Target)
	}
	c.JSON(http.StatusOK, gin.H{"message": "Action undone"})
}
