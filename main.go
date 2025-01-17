package main

import (
	"fmt"
	"log"
	"macaoapply-auto/internal/app"
	"macaoapply-auto/internal/model"
	"macaoapply-auto/internal/router"
	"macaoapply-auto/pkg/config"
	"net/http"
	"os/exec"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func init() {
	//gin.SetMode(gin.ReleaseMode)
}

func SetupWs() {
	//go ws.InitWs()
}

type WriterProxy struct{}

var msgChan = make(chan string, 100) // 100条消息缓冲

func (w *WriterProxy) Write(p []byte) (n int, err error) {
	fmt.Print(string(p))
	msgChan <- string(p)
	return len(p), nil
}

var upgrader = websocket.Upgrader{
	// 解决跨域问题
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
} // use default options

type ClientManager struct {
	clients map[*websocket.Conn]bool
	lock    sync.Mutex
}

func (cm *ClientManager) Add(ws *websocket.Conn) {
	cm.lock.Lock()
	defer cm.lock.Unlock()
	cm.clients[ws] = true
}

func (cm *ClientManager) Remove(ws *websocket.Conn) {
	cm.lock.Lock()
	defer cm.lock.Unlock()
	delete(cm.clients, ws)
}

func (cm *ClientManager) WriteMessage(message string) {
	cm.lock.Lock()
	defer cm.lock.Unlock()
	for ws := range cm.clients {
		if err := ws.WriteMessage(websocket.TextMessage, []byte(message)); err != nil {
			log.Println("write:", err)
			cm.lock.Lock()
			delete(cm.clients, ws)
			cm.lock.Unlock()
		}
	}
}

var clientManager *ClientManager

func serveWS(c *gin.Context) {
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("upgrade:", err)
		return
	}
	defer ws.Close()
	clientManager.Add(ws)
	defer clientManager.Remove(ws)
	// hello
	if err := ws.WriteMessage(websocket.TextMessage, []byte("hello")); err != nil {
		log.Println("write:", err)
		return
	}
	for {
		// 断开检测
		_, _, err := ws.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			break
		}
	}
}

func wsWriter() {
	for {
		msg := <-msgChan
		clientManager.WriteMessage(msg)
	}
}

func reboot() {
	log.Println("reboot")
	app.BootStrap()
}

func main() {
	// setup log websocket
	writerProxy := &WriterProxy{}
	clientManager = &ClientManager{
		clients: make(map[*websocket.Conn]bool),
		lock:    sync.Mutex{},
	}
	log.SetOutput(writerProxy)
	go wsWriter()
	// setup model
	model.Setup()
	server := router.InitRouter()
	server.GET("/ws", serveWS)
	// webui
	server.Static("/webui", "./webui")
	webuiUrl := "http://localhost:" + config.Config.Port + "/webui"
	// 打开浏览器
	go func() {
		time.Sleep(time.Second)
		exec.Command("cmd", "/c", "start", webuiUrl).Start()
		// linux
		exec.Command("xdg-open", webuiUrl).Start()
	}()
	// go app.BootStrap()
	port := config.Config.Port
	log.Println("macaoapply-auto start success, listen on " + port)
	err := server.Run(":" + port)
	if err != nil {
		return
	}
}
