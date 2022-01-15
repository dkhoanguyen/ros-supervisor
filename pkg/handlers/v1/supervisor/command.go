package supervisor

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
)

type SupervisorCommand struct {
	UpdateCore     bool `json:"update_core"`
	UpdateServices bool `json:"update_services"`
}

func MakeCommand(parentCtx context.Context, cmd *SupervisorCommand) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := c.BindJSON(&cmd); err != nil {
			fmt.Println(err)
		}
	}
}
