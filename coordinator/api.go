package coordinator

import (
	"context"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
)

func ServiceRegisterHandler(c *gin.Context) {
	if state.backupContext != nil {
		c.JSON(200, gin.H{
			"status": "backup is running",
		})
		return
	}
}

func BackupHandler(c *gin.Context) {
	if state.backupContext != nil {
		c.JSON(200, gin.H{
			"status": "backup is running",
		})
		return
	}
	state.backStatusLock.Lock()
	if time.Since(state.lastSuccessBackupTime) <= 5*time.Minute {
		c.JSON(200, gin.H{
			"status": fmt.Sprintf("backup ran recently, last backup time: %v", state.lastSuccessBackupTime.Format(time.RFC1123Z)),
		})
		state.backStatusLock.Unlock()
		return
	}
	state.backupContext, state.backupCancelFn = context.WithCancel(context.Background())
	go startBackup(state.backupContext)
	c.JSON(200, gin.H{
		"status": "backup started",
	})
}

func BackupStatusHandler(c *gin.Context) {

}

func CancelBackupHandler(c *gin.Context) {
	if state.backupContext == nil {
		c.JSON(200, gin.H{
			"status": "backup is not running",
		})
		return
	}

	state.backupCancelFn()

	c.JSON(200, gin.H{
		"status": "backup canceled",
	})
	return
}
