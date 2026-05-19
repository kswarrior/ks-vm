package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"ksvm/pkg/kvm"
)

type API struct {
	manager *kvm.Manager
}

func New(m *kvm.Manager) *API {
	return &API{manager: m}
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
	}
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
		Name  string `json:"name" binding:"required"`
		Image string `json:"image" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := a.manager.Deploy(req.Name, req.Image); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "Deployed " + req.Name})
}

func (a *API) launchInstance(c *gin.Context) {
	name := c.Param("name")
	if err := a.manager.Launch(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Started " + name})
}

func (a *API) stopInstance(c *gin.Context) {
	name := c.Param("name")
	if err := a.manager.Stop(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Stopped " + name})
}

func (a *API) restartInstance(c *gin.Context) {
	name := c.Param("name")
	if err := a.manager.Restart(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Restarted " + name})
}

func (a *API) suspendInstance(c *gin.Context) {
	name := c.Param("name")
	if err := a.manager.Suspend(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Suspended " + name})
}

func (a *API) resumeInstance(c *gin.Context) {
	name := c.Param("name")
	if err := a.manager.Resume(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
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
	c.JSON(http.StatusOK, gin.H{"message": "Updated " + name})
}

func (a *API) deleteInstance(c *gin.Context) {
	name := c.Param("name")
	if err := a.manager.Delete(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
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
	c.JSON(http.StatusOK, []gin.H{
		{"username": "admin", "email": "admin@ksvm.local"},
	})
}

func (a *API) createUser(c *gin.Context) {
	c.JSON(http.StatusCreated, gin.H{"message": "User created"})
}

func (a *API) getAuditLogs(c *gin.Context) {
	c.JSON(http.StatusOK, []gin.H{
		{"timestamp": "2025-05-18 14:20:10", "user": "admin", "action": "deploy", "target": "ubuntu-web"},
	})
}
