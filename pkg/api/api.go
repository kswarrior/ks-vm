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
		v1.POST("/images", a.addImage)
		v1.PUT("/images/rename", a.renameImage)
		v1.DELETE("/images/:name", a.removeImage)
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
		v1.POST("/ssh/:name", a.setupSSH)
		v1.POST("/exec/:name", a.runCommand)

		// System & Users
		v1.GET("/monitor", a.getMonitorData)
		v1.GET("/users", a.listUsers)
		v1.POST("/users", a.createUser)
		v1.DELETE("/users/:username", a.deleteUser)
		v1.GET("/logs", a.getAuditLogs)
		v1.POST("/logs/undo/:id", a.undoAction)
	}
}

func (a *API) addLog(c *gin.Context, action, target string) {
	a.lMu.Lock()
	defer a.lMu.Unlock()

	user, _, _ := c.Request.BasicAuth()
	if user == "" {
		user = "system"
	}

	entry := LogEntry{
		ID:        len(a.logs),
		Timestamp: time.Now().Format("2006-01-02 15:04:05"),
		User:      user,
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
		Type     string `json:"type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	opts := kvm.DeployOptions{
		User:         req.User,
		Password:     req.Password,
		CPUs:         req.CPUs,
		MemoryMB:     req.MemoryMB,
		DiskGB:       req.DiskGB,
		InstanceType: req.Type,
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
	oldName := c.Param("name")
	var req struct {
		Name     string `json:"name"`
		MemoryMB uint   `json:"memory_mb"`
		CPUs     uint   `json:"cpus"`
		User     string `json:"user"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	opts := kvm.DeployOptions{
		User:     req.User,
		Password: req.Password,
		MemoryMB: req.MemoryMB,
		CPUs:     req.CPUs,
	}

	if err := a.manager.UpdateInstance(oldName, req.Name, opts); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	a.addLog(c, "update", oldName)
	c.JSON(http.StatusOK, gin.H{"message": "Updated " + oldName})
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

func (a *API) setupSSH(c *gin.Context) {
	name := c.Param("name")
	var req struct {
		Port string `json:"port"`
		URL  string `json:"url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		// Fallback for older clients or default behavior
		req.Port = "3030"
	}

	token, err := a.manager.SetupSSH(name, req.Port, req.URL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	a.addLog(c, "ssh", name)
	c.JSON(http.StatusOK, gin.H{"token": token})
}

func (a *API) runCommand(c *gin.Context) {
	name := c.Param("name")
	var req struct {
		Command string `json:"command" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	output, err := a.manager.Exec(name, []string{"/bin/sh", "-c", req.Command})
	a.addLog(c, "exec", name)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"output": output, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"output": output})
}

func (a *API) getMonitorData(c *gin.Context) {
	metrics, err := kvm.GetHostMetrics()
	if err != nil {
		// Log the error and return empty structure to keep UI alive
		metrics = kvm.HostMetrics{}
	}

	instances, _ := a.manager.List()

	active := 0
	for _, inst := range instances {
		if inst.Status == "running" {
			active++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"metrics":          metrics,
		"active_instances": active,
	})
}

func (a *API) listUsers(c *gin.Context) {
	a.uMu.RLock()
	defer a.uMu.RUnlock()

	// Filter out sensitive data
	users := make([]User, len(a.users))
	for i, u := range a.users {
		users[i] = User{
			Username:    u.Username,
			Email:       u.Email,
			Permissions: u.Permissions,
		}
	}
	c.JSON(http.StatusOK, users)
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

func (a *API) addImage(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
		URL  string `json:"url" binding:"required"`
		Type string `json:"type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := a.manager.AddImage(req.Name, req.URL, req.Type); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	a.addLog(c, "add_image", req.Name)
	c.JSON(http.StatusCreated, gin.H{"message": "Image added"})
}

func (a *API) renameImage(c *gin.Context) {
	var req struct {
		OldName string `json:"old_name" binding:"required"`
		NewName string `json:"new_name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Try common extensions
	exts := []string{"", ".qcow2", ".lxd"}
	found := false
	var oldPath, newPath string

	for _, ext := range exts {
		p := filepath.Join(kvm.ImagesDir, req.OldName+ext)
		if _, err := os.Stat(p); err == nil {
			oldPath = p
			newPath = filepath.Join(kvm.ImagesDir, req.NewName+ext)
			found = true
			break
		}
	}

	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "Image file not found"})
		return
	}

	if err := os.Rename(oldPath, newPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	a.addLog(c, "rename_image", req.OldName+" -> "+req.NewName)
	c.JSON(http.StatusOK, gin.H{"message": "Image renamed"})
}

func (a *API) removeImage(c *gin.Context) {
	name := c.Param("name")
	if err := a.manager.RemoveImage(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	a.addLog(c, "remove_image", name)
	c.JSON(http.StatusOK, gin.H{"message": "Image removed"})
}

func (a *API) deleteUser(c *gin.Context) {
	username := c.Param("username")
	if username == "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Master user cannot be deleted"})
		return
	}
	a.uMu.Lock()
	defer a.uMu.Unlock()
	for i, u := range a.users {
		if u.Username == username {
			a.users = append(a.users[:i], a.users[i+1:]...)
			a.saveUsers()
			a.addLog(c, "delete_user", username)
			c.JSON(http.StatusOK, gin.H{"message": "User deleted"})
			return
		}
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
}
