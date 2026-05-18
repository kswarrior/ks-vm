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
