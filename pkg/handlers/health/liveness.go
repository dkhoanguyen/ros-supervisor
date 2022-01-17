package health

import "github.com/gin-gonic/gin"

func LivenessGet(c *gin.Context) {
	c.JSON(200, gin.H{
		"status": "UP",
	})
}
