package main

import (
	"log"
	"net/http"
    "crypto/md5"
    "crypto/rand"
    "encoding/hex"
    "fmt"
    "io"
    "io/ioutil"
    "strings"
    "bytes"
    "encoding/json"
	"github.com/gin-gonic/gin"
    mrand "math/rand"
	"time"
    "os"
    "os/exec"
    "bufio"
    "encoding/base64"
    "strconv"
)

type SnapshotRequest struct {
	Id string `json:"id"`
	Url string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type VideoUpload struct {
	Id string `json:"id"`
	Data string `json:"data"`
}

var cmd* exec.Cmd
var configs = []string{"Brightness", "ChromaSuppress", "Contrast", "Gamma", "Hue", "Saturation"}

func StartRecording(c *gin.Context) {
    url, port, username, password := Config.GetRtspParams(c.Param("suuid"))
    var name string
    for {
        mrand.NewSource(time.Now().UnixNano())
        name = "temp/" + strconv.Itoa(mrand.Intn(999999)) + ".mkv"
        if _, err := os.Stat(name); err != nil {
            break
        }
    }
    os.MkdirAll("temp", os.ModePerm)
    cmd = exec.Command("ffmpeg", "-rtsp_transport", "tcp", "-i", "rtsp://" + username + ":" + password + "@" + url + ":" + port + "/cam/realmonitor?channel=1&subtype=0", "-acodec", "copy", "-vcodec", "copy", name)
	
	if err := cmd.Start(); err != nil {
		log.Println(err)
        c.AbortWithStatus(http.StatusBadRequest)
		return
	}
    fileName = name
    c.JSON(http.StatusOK, "ok")
}

func StopRecording(c *gin.Context) {
	if err := cmd.Process.Kill(); err != nil {
		log.Println(err)
		return
	}

    f, _ := os.Open(fileName)
    
    reader := bufio.NewReader(f)
    content, _ := ioutil.ReadAll(reader)
    
    encoded := base64.StdEncoding.EncodeToString(content)
	os.Remove(fileName)

    fileName = ""
	
	cookie, _ := c.Request.Cookie("Authorization")
	value := cookie.Value
    claims := GetClaims(value)
	videoUpload := &VideoUpload{
		Id: claims.Id,
		Data: encoded,
	}
	json_data, err := json.Marshal(videoUpload)
	req, err := http.NewRequest(http.MethodPost, monitorURL + "/api/CameraHub/Video", bytes.NewReader(json_data))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", secret)

	resp, err := http.DefaultClient.Do(req)
    
	if err != nil || resp.StatusCode >= 400 {
      c.AbortWithStatus(http.StatusBadRequest)
    }
    c.JSON(http.StatusOK, "ok")
}

func TakeSnapshot(c *gin.Context) {
    url, port, username, password := Config.GetHttpParams(c.Param("suuid"))
	cookie, _ := c.Request.Cookie("Authorization")
	value := cookie.Value
    claims := GetClaims(value)
	snapshotRequest := &SnapshotRequest{
		Id: claims.Id,
		Url: "http://" + url + ":" + port + "/cgi-bin/snapshot.cgi",
		Username: username,
		Password: password,
	}
	json_data, err := json.Marshal(snapshotRequest)
	req, err := http.NewRequest(http.MethodPost, monitorURL + "/api/CameraHub/Snapshot", bytes.NewReader(json_data))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", secret)

	resp, err := http.DefaultClient.Do(req)

	if err != nil || resp.StatusCode >= 400 {
      c.AbortWithStatus(http.StatusBadRequest)
    }
    c.JSON(http.StatusOK, "ok")
}

func CameraMoveControl(c *gin.Context) {
    url, port, username, password := Config.GetHttpParams(c.Param("suuid"))
	log.Println(digestPost("http://" + url + ":" + port + "/", "cgi-bin/ptz.cgi?action=" + c.Param("action") + "&channel=1&code=" + c.Param("direction") + "&arg1=" + c.Param("speed") + "&arg2=" + c.Param("speed") + "&arg3=0", username, password))
}

func ChangeConfig(c *gin.Context) {
    url, port, username, password := Config.GetHttpParams(c.Param("suuid"))
	log.Println(digestPost("http://" + url + ":" + port + "/", "cgi-bin/configManager.cgi?action=setConfig&VideoColor[0][0]." + c.Param("config") + "=" + c.Param("value"), username, password))
}

func ResetAllConfigs() {
	_, all := Config.list()
    var path = "cgi-bin/configManager.cgi?action=setConfig"
    for _, config := range configs{
        path += "&VideoColor[0][0]." + config + "=50"
    }
    for _, ssuid := range all{
        url, port, username, password := Config.GetHttpParams(ssuid)
	    go digestPost("http://" + url + ":" + port + "/", path, username, password)
    }
}

func ResetConfigs(c *gin.Context) {
    var path = "cgi-bin/configManager.cgi?action=setConfig"
    for _, config := range configs{
        path += "&VideoColor[0][0]." + config + "=50"
    }
    url, port, username, password := Config.GetHttpParams(c.Param("suuid"))
	digestPost("http://" + url + ":" + port + "/", path, username, password)
}

func GetConfig(c *gin.Context) {
    url, port, username, password := Config.GetHttpParams(c.Param("suuid"))
    success, resp := digestPost("http://" + url + ":" + port + "/", "cgi-bin/configManager.cgi?action=getConfig&name=VideoColor", username, password)
    if(success){
        resp = strings.Replace(strings.Split(resp, "table.VideoColor[0][0].Style")[0], "table.VideoColor[0][0].", "", -1)
        c.String(http.StatusOK, resp)
    }
}

func digestPost(host string, uri string, username string, password string) (bool, string) {
    url := host + uri
    method := "POST"
    req, err := http.NewRequest(method, url, nil)
    req.Header.Set("Content-Type", "application/json")
    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        return false, err.Error()
    }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusUnauthorized {
        log.Printf("Recieved status code '%v' auth skipped", resp.StatusCode)
        return true, ""
    }
    digestParts := digestParts(resp)
    digestParts["uri"] = uri
    digestParts["method"] = method
    digestParts["username"] = username
    digestParts["password"] = password
    req, err = http.NewRequest(method, url, nil)
    req.Header.Set("Authorization", getDigestAuthrization(digestParts))
    req.Header.Set("Content-Type", "application/json")

    resp, err = client.Do(req)
    if err != nil {
        return false, err.Error()
    }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusOK {
        body, err := ioutil.ReadAll(resp.Body)
        if err != nil {
            return false, err.Error()
        }
        return false, string(body)
    } else {
        body, err := ioutil.ReadAll(resp.Body)
        if err != nil {
            return true, ""
        }
        return true, string(body)
    }
}

func digestParts(resp *http.Response) map[string]string {
    result := map[string]string{}
    if len(resp.Header["Www-Authenticate"]) > 0 {
        wantedHeaders := []string{"nonce", "realm", "qop"}
        responseHeaders := strings.Split(resp.Header["Www-Authenticate"][0], ",")
        for _, r := range responseHeaders {
            for _, w := range wantedHeaders {
                if strings.Contains(r, w) {
                    result[w] = strings.Split(r, `"`)[1]
                }
            }
        }
    }
    return result
}

func getDigestAuthrization(digestParts map[string]string) string {
    d := digestParts
    ha1 := getMD5(d["username"] + ":" + d["realm"] + ":" + d["password"])
    ha2 := getMD5(d["method"] + ":" + d["uri"])
    nonceCount := 00000001
    cnonce := getCnonce()
    response := getMD5(fmt.Sprintf("%s:%s:%v:%s:%s:%s", ha1, d["nonce"], nonceCount, cnonce, d["qop"], ha2))
    authorization := fmt.Sprintf(`Digest username="%s", realm="%s", nonce="%s", uri="%s", cnonce="%s", nc="%v", qop="%s", response="%s"`,
        d["username"], d["realm"], d["nonce"], d["uri"], cnonce, nonceCount, d["qop"], response)
    return authorization
}

func getMD5(text string) string {
    hasher := md5.New()
    hasher.Write([]byte(text))
    return hex.EncodeToString(hasher.Sum(nil))
}

func getCnonce() string {
    b := make([]byte, 8)
    io.ReadFull(rand.Reader, b)
    return fmt.Sprintf("%x", b)[:16]
}