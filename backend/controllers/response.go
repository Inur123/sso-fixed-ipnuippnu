package controllers

import "github.com/gin-gonic/gin"

func respondError(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{"error": code, "message": message})
}
