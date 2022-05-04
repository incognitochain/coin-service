package coordinator

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/incognitochain/coin-service/coordinator/detector"
)

var upGrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func ServiceRegisterHandler(c *gin.Context) {
	if state.backupContext != nil {
		c.JSON(200, gin.H{
			"status": "backup is running",
		})
		return
	}
	ws, err := upGrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("error get connection")
		log.Fatal(err)
	}
	defer ws.Close()
	readCh := make(chan []byte)
	writeCh := make(chan []byte)
	serviceID := ""
	serviceGroup := ""
	gitCommit := ""
	if len(c.Request.Header.Values("id")) > 0 {
		serviceID = c.Request.Header.Values("id")[0]
	}
	if len(c.Request.Header.Values("service")) > 0 {
		serviceGroup = c.Request.Header.Values("service")[0]
	}
	if len(c.Request.Header.Values("gitcommit")) > 0 {
		gitCommit = c.Request.Header.Values("gitcommit")[0]
	}
	if serviceID == "" || serviceGroup == "" {
		c.JSON(200, gin.H{
			"status": "service id or service name is empty",
		})
		return
	}

	newService := new(ServiceConn)
	newService.ID = serviceID
	newService.ServiceGroup = serviceGroup
	newService.GitCommit = gitCommit
	newService.ReadCh = readCh
	newService.WriteCh = writeCh
	newService.ConnectedTime = time.Now().Unix()
	newService.IsPause = true
	done := make(chan struct{})
	newService.closeCh = done

	go func() {
		for {
			select {
			case <-done:
				removeService(newService)
				close(writeCh)
				ws.Close()
				return
			default:
				_, msg, err := ws.ReadMessage()
				if err != nil {
					log.Println(err)
					close(done)
					return
				}
				if len(msg) == 1 {
					continue
				}
				var cmd CoordinatorCmd
				err = json.Unmarshal(msg, &cmd)
				if err != nil {
					log.Println(err)
					continue
				}
				switch cmd.Action {
				case ACTION_OPERATION_STATUS:
					if cmd.Data == "pause" {
						newService.IsPause = true
					} else {
						newService.IsPause = false
					}
				}
			}
		}
	}()

	err = registerService(newService)
	if err != nil {
		log.Println(err)
		return
	}
	for {
		select {
		case <-done:
			crashRecord := detector.RecordDetail{
				ServiceID: newService.ID,
				Time:      time.Now().Unix(),
				Type:      detector.RECORDTYPE_LOSTCONNECTION,
				Reason:    "unknown",
			}
			state.Detector.AddRecord(crashRecord, serviceGroup)
			removeService(newService)
			go suspectCrash(serviceID, serviceGroup)
			return
		case msg := <-writeCh:
			fmt.Println("writeCh", string(msg))
			err := ws.WriteMessage(websocket.TextMessage, msg)
			if err != nil {
				log.Println("write:", err)
				continue
			}
		}
	}
}

func BackupHandler(c *gin.Context) {
	if state.backupContext != nil {
		if state.currentBackupProgress != nil {
			cur, m := state.currentBackupProgress.GetProgressStatus()
			c.JSON(200, gin.H{
				"status":   "backup is running",
				"progress": fmt.Sprintf("%v/%v", cur, m),
			})
			return
		} else {
			c.JSON(200, gin.H{
				"status": "backup is initailizing",
			})
			return
		}
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
	state.backStatusLock.Unlock()
	go startBackup()
	c.JSON(200, gin.H{
		"status": "backup started",
	})
}

func BackupStatusHandler(c *gin.Context) {
	if state.backupContext == nil {
		c.JSON(200, gin.H{
			"status":            "backup is not running",
			"statusCode":        state.backupState,
			"lastSuccessBackup": state.lastSuccessBackupTime.Format(time.RFC1123Z),
		})
		return
	}
	if state.currentBackupProgress == nil {
		c.JSON(200, gin.H{
			"status":            "backup is initailizing",
			"statusCode":        state.backupState,
			"lastSuccessBackup": state.lastSuccessBackupTime.Format(time.RFC1123Z),
		})
		return
	}

	cur, m := state.currentBackupProgress.GetProgressStatus()
	c.JSON(200, gin.H{
		"status":            "backup is running",
		"statusCode":        state.backupState,
		"progress":          fmt.Sprintf("%v/%v", cur, m),
		"lastSuccessBackup": state.lastSuccessBackupTime.Format(time.RFC1123Z),
	})
	return

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

func GetServiceStatusHandler(c *gin.Context) {
	state.ConnectedServicesLock.RLock()
	defer state.ConnectedServicesLock.RUnlock()
	type ServiceStatus struct {
		ID            string
		IsPause       bool
		GitCommit     string
		ConnectedTime int64
	}

	serviceStats := make(map[string][]ServiceStatus)
	for k, instances := range state.ConnectedServices {
		for in, v := range instances {
			serviceStats[k] = append(serviceStats[k], ServiceStatus{
				ID:            in,
				IsPause:       v.IsPause,
				GitCommit:     v.GitCommit,
				ConnectedTime: v.ConnectedTime,
			})
		}

	}
	c.JSON(200, gin.H{
		"services": serviceStats,
	})
}

func ServiceListHandler(c *gin.Context) {

}

func ListBackupsHandler(c *gin.Context) {
	pwd, _ := os.Getwd()
	files, err := ioutil.ReadDir(path.Join(pwd, "mongodump"))
	if err != nil {
		log.Fatal(err)
	}

	type FileInfo struct {
		Name    string
		Size    int64
		ModTime time.Time
	}
	var fileList []FileInfo
	for _, file := range files {
		fileList = append(fileList, FileInfo{
			Name:    file.Name(),
			Size:    file.Size(),
			ModTime: file.ModTime(),
		})
	}
	c.JSON(200, gin.H{
		"backups": fileList,
	})
}

func CrashSummaryHandler(c *gin.Context) {
	crashCount, total := state.Detector.GetCrashCountAll()
	var result CrashSummary
	result.Total = total

	result.ByService = make(map[string]int)
	result.ByType = make(map[string]int)

	for service, tc := range crashCount {
		for crashType, v := range tc {
			result.ByService[service] += v
			result.ByType[crashType] += v
		}
	}

	c.JSON(200, gin.H{
		"result": result,
	})
}

func ServiceCrashDetailHandler(c *gin.Context) {
	result := state.Detector.GetCrashReportByService(c.Param("service"))
	c.JSON(200, gin.H{
		"result": result,
	})
}

func suspectCrash(serviceID, serviceGroup string) {
	time.Sleep(60 * time.Second)
	state.ConnectedServicesLock.RLock()
	if _, ok := state.ConnectedServices[serviceID]; !ok {
		crashRecord := detector.RecordDetail{
			ServiceID: serviceID,
			Time:      time.Now().Unix(),
			Type:      detector.RECORDTYPE_SUSPECT_CRASH,
			Reason:    "unknown",
		}
		state.Detector.AddRecord(crashRecord, serviceGroup)
	}
}
