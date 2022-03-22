package apiservice

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/incognitochain/coin-service/database"
	"net/http"
)

func APIGetDeviceByQRCode(c *gin.Context) {
	var req struct {
		QRCode string `form:"QRCode" json:"QRCode" binding:"required"`
	}
	err := c.ShouldBindQuery(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}

	device, err := database.DBGetDeviceByDeviceQRCode(req.QRCode)

	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}

	if device == nil {
		c.JSON(http.StatusNotFound, buildGinErrorRespond(errors.New("qrcode not found")))
		return
	}

	respond := APIRespond{
		Result: device,
		Error:  nil,
	}
	c.JSON(http.StatusOK, respond)
}
