package main

import (
	"log"
	"net/http"
	"sort"
	"time"

	"github.com/deepch/vdk/av"
	"github.com/deepch/vdk/format/mp4f"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/websocket"
)

const monitorURL = "http://194.210.120.34:5000"
var fileName = ""

func serveHTTP() {
	router := gin.Default()
	gin.SetMode(gin.DebugMode)
	router.LoadHTMLGlob("web/templates/*")
	

	router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "login.html", gin.H{
			"version":  time.Now().String(),
		})
	})
	router.POST("/login", func(c *gin.Context) {
		if !Authenticate(c){
			c.AbortWithStatus(http.StatusUnauthorized)
		} else{
			ResetAllConfigs()
		    c.JSON(http.StatusOK, "ok")
		}
	})
	router.GET("/player", func(c *gin.Context) {
		if cookie, err := c.Request.Cookie("Authorization"); err == nil {
			value := cookie.Value
			if Authorize(value) {
				fi, all := Config.list()
				sort.Strings(all)
				c.HTML(http.StatusOK, "index.tmpl", gin.H{
					"port":     Config.Server.HTTPPort,
					"suuid":    fi,
					"suuidMap": all,
					"version":  time.Now().String(),
				})
				return
			}
		}
		c.Redirect(http.StatusFound, "/")
	})
	router.GET("/player/:suuid", func(c *gin.Context) {
		if cookie, err := c.Request.Cookie("Authorization"); err == nil {
			value := cookie.Value
			if Authorize(value) {
				_, all := Config.list()
				sort.Strings(all)
				c.HTML(http.StatusOK, "index.tmpl", gin.H{
					"port":     Config.Server.HTTPPort,
					"suuid":    c.Param("suuid"),
					"suuidMap": all,
					"version":  time.Now().String(),
				})
				return
			}
		}
		c.Redirect(http.StatusFound, "/")
	})
	router.GET("/ws/:suuid", func(c *gin.Context) {
		if cookie, err := c.Request.Cookie("Authorization"); err == nil {
			value := cookie.Value
			if Authorize(value) {
				handler := websocket.Handler(ws)
				handler.ServeHTTP(c.Writer, c.Request)
				log.Println(2)
				if fileName != ""{
					StopRecording(c)
				}
				return
			}
		}
		c.Redirect(http.StatusFound, "/")
	})
	router.GET("/:suuid/move/:action/:direction/:speed", func(c *gin.Context) {
		if cookie, err := c.Request.Cookie("Authorization"); err == nil {
			value := cookie.Value
			if Authorize(value) {
				CameraMoveControl(c)
				return
			}
		}
		c.Redirect(http.StatusFound, "/")
	})
	router.GET("/:suuid/video/:action", func(c *gin.Context) {
		if cookie, err := c.Request.Cookie("Authorization"); err == nil {
			value := cookie.Value
			if Authorize(value) {
				action := c.Param("action")
				if action == "start"{
					if fileName == ""{
						StartRecording(c)
					}
				} else if action == "stop"{
					if fileName != ""{
						StopRecording(c)
					}
				} else{

				}
				return
			}
		}
		c.Redirect(http.StatusFound, "/")
	})
	router.GET("/:suuid/snapshot", func(c *gin.Context) {
		if cookie, err := c.Request.Cookie("Authorization"); err == nil {
			value := cookie.Value
			if Authorize(value) {
				TakeSnapshot(c)
				return
			}
		}
		c.Redirect(http.StatusFound, "/")
	})
	router.GET("/:suuid/config/:config/:value", func(c *gin.Context) {
		if cookie, err := c.Request.Cookie("Authorization"); err == nil {
			value := cookie.Value
			if Authorize(value) {
				ChangeConfig(c)
				return
			}
		}
		c.Redirect(http.StatusFound, "/")
	})
	router.GET("/:suuid/config/reset", func(c *gin.Context) {
		if cookie, err := c.Request.Cookie("Authorization"); err == nil {
			value := cookie.Value
			if Authorize(value) {
				ResetConfigs(c)
				GetConfig(c)
				return
			}
		}
		c.Redirect(http.StatusFound, "/")
	})
	router.GET("/:suuid/config", func(c *gin.Context) {
		if cookie, err := c.Request.Cookie("Authorization"); err == nil {
			value := cookie.Value
			if Authorize(value) {
				GetConfig(c)
				return
			}
		}
		c.Redirect(http.StatusFound, "/")
	})
	router.StaticFS("/static", http.Dir("web/static"))
	err := router.Run(Config.Server.HTTPPort)
	if err != nil {
		log.Fatalln(err)
	}
}
func ws(ws *websocket.Conn) {
	defer ws.Close()
	suuid := ws.Request().FormValue("suuid")
	log.Println("Request", suuid)
	if !Config.ext(suuid) {
		log.Println("Stream Not Found")
		return
	}
	Config.RunIFNotRun(suuid)
	ws.SetWriteDeadline(time.Now().Add(5 * time.Second))
	cuuid, ch := Config.clAd(suuid)
	defer Config.clDe(suuid, cuuid)
	codecs := Config.coGe(suuid)
	if codecs == nil {
		log.Println("Codecs Error")
		return
	}
	for i, codec := range codecs {
		if codec.Type().IsAudio() && codec.Type() != av.AAC {
			log.Println("Track", i, "Audio Codec Work Only AAC")
		}
	}
	muxer := mp4f.NewMuxer(nil)
	err := muxer.WriteHeader(codecs)
	if err != nil {
		log.Println("muxer.WriteHeader", err)
		return
	}
	meta, init := muxer.GetInit(codecs)
	err = websocket.Message.Send(ws, append([]byte{9}, meta...))
	if err != nil {
		log.Println("websocket.Message.Send", err)
		return
	}
	err = websocket.Message.Send(ws, init)
	if err != nil {
		return
	}
	var start bool
	go func() {
		for {
			var message string
			err := websocket.Message.Receive(ws, &message)
			if err != nil {
				ws.Close()
				return
			}
		}
	}()
	noVideo := time.NewTimer(10 * time.Second)
	var timeLine = make(map[int8]time.Duration)
	t := time.Now()
	for {
		if t.Before(time.Now()) {
			cookie, err := ws.Request().Cookie("Authorization")
			if err != nil || !Authorize(cookie.Value){
				return
			}
			t = time.Now().Add(time.Minute)
		}
		select {
		case <-noVideo.C:
			log.Println("noVideo")
			return
		case pck := <-ch:
			if pck.IsKeyFrame {
				noVideo.Reset(10 * time.Second)
				start = true
			}
			if !start {
				continue
			}
			timeLine[pck.Idx] += pck.Duration
			pck.Time = timeLine[pck.Idx]
			ready, buf, _ := muxer.WritePacket(pck, false)
			if ready {
				err = ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err != nil {
					return
				}
				err := websocket.Message.Send(ws, buf)
				if err != nil {
					return
				}
			}
		}
	}
}
