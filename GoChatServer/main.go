package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

type Client struct {
	ID     string
	Avatar string
	Conn   *websocket.Conn
}

type Message struct {
	Sender   string `json:"sender"`
	Receiver string `json:"receiver"`
	Command  string `json:"command"` // 用于区分服务器命令或转发
	Data     string `json:"data"`    // 消息内容
}

var (
	upgrader     = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	clients      = make(map[string]*Client) // 存储所有连接的客户端
	clientsMutex sync.Mutex                 // 保护客户端列表的并发安全
)

func main() {
	http.HandleFunc("/ws", handleConnections)
	log.Println("启动 WebSocket 服务器，监听端口 8008...")
	//开一个线程来读取命令行的广播消息
	go func() {
		reader := bufio.NewReader(os.Stdin)
		for {
			fmt.Print("请输入你需要广播的内容： ")
			input, err := reader.ReadString('\n') // 读取直到换行符
			if err != nil {
				fmt.Println("读取输入失败:", err)
				continue
			}

			// 去除输入字符串的换行符和多余空白
			input = strings.TrimSpace(input)

			// 如果输入有效，则广播消息
			if input != "" {
				broadNormalMessage(input, nil)
			}
		}
	}()
	log.Fatal(http.ListenAndServe(":8008", nil))
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("升级为 WebSocket 失败:", err)
		return
	}
	defer conn.Close()

	// 获取用户 ID
	userID := r.URL.Query().Get("id")
	if userID == "" {
		userID = fmt.Sprintf("用户-%d", len(clients)+1)
	}
	avatar := r.URL.Query().Get("avatar")

	// 将用户添加到客户端列表
	client := &Client{ID: userID, Avatar: avatar, Conn: conn}
	clientsMutex.Lock()
	clients[userID] = client
	clientsMutex.Unlock()
	log.Printf("新用户连接: %s\n", userID)

	//-----------------------------------------------
	broadNormalMessage("Welcome to My Go ChatApplication", client)
	//-----------------------------------------------
	// 新加入的用户也需要广播用户列表
	broadcastUserList()

	// 监听客户端消息
	OnReceiveMes(client)

	// 用户断开连接时，移除用户
	clientsMutex.Lock()
	delete(clients, userID)
	clientsMutex.Unlock()
	log.Printf("用户 %s 已断开连接\n", userID)

	// 广播更新的用户列表
	broadcastUserList()

}
func forwardToUser(sender string, receiver string, message string) {
	clientsMutex.Lock()
	defer clientsMutex.Unlock()

	// 检查目标用户是否在线
	client, ok := clients[receiver]
	if !ok {
		log.Printf("用户 %s 不在线，消息未发送: %s\n", receiver, message)
		go forwardToUser(receiver, sender, "The User has been OFF-LINE,please leave the page! ")
		return
	}

	// 创建消息结构
	responseMessage := Message{
		Sender:   sender,
		Receiver: receiver,
		Command:  sender,
		Data:     message,
	}

	// 将消息转换为 JSON
	response, err := json.Marshal(responseMessage)
	if err != nil {
		log.Println("生成消息 JSON 失败:", err)
		return
	}

	// 向目标用户发送消息
	err = client.Conn.WriteMessage(websocket.TextMessage, response)
	if err != nil {
		log.Printf("向用户 %s 发送消息失败: %v\n", receiver, err)
	} else {
		log.Printf("成功转发消息给用户 %s: %s\n", receiver, message)
	}
}

func OnReceiveMes(client *Client) {
	// 读取客户端消息
	for {
		_, rawMessage, err := client.Conn.ReadMessage()
		if err != nil {
			log.Printf("用户断开连接: %s, 错误: %v\n", client.ID, err)
			break
		}
		log.Printf("收到来自 %s 的消息: %s\n", client.ID, string(rawMessage))
		// 解析 JSON 消息
		var msg Message
		err = json.Unmarshal(rawMessage, &msg)
		if err != nil {
			log.Printf("解析消息出错: %v\n", err)
			continue
		}
		// 根据 Receiver 字段处理逻辑
		switch msg.Receiver {
		case "server":
			// 处理服务器消息
			log.Printf("处理用户发来的服务器消息: %s\n", msg)
			//handleServerCommand(msg.Data)
			if msg.Command == "UpdateAvatar" {
				//如果是更新头像
				clientsMutex.Lock()
				clients[msg.Sender].Avatar = msg.Data
				clientsMutex.Unlock()
				broadcastUserList()
			}
		default:
			// 转发给其他用户
			log.Printf("转发消息给其他用户: %s\n", msg)
			forwardToUser(msg.Sender, msg.Receiver, msg.Data)
		}

	}
}

func broadcastUserList() {
	clientsMutex.Lock()
	defer clientsMutex.Unlock()

	// if len(clients) < 2 {
	// 	return
	// }

	for _, client := range clients {
		// 当前客户端的 ID
		excludeUser := client.ID

		// 构建用户列表，排除当前客户端
		var userList []string
		for id := range clients {
			if id != excludeUser {
				//这里用@作为分隔符，前面是用户ID，后面是用户头像
				userList = append(userList, id+"@"+clients[id].Avatar)
			}
		}
		// 检查是否有用户
		userListString := ""
		if len(userList) > 0 {
			userListString = strings.Join(userList, ",")
		}
		// 创建消息结构
		message := Message{
			Sender:   "server",
			Receiver: client.ID,
			Command:  "userList",
			Data:     userListString, // 将用户列表转换为字符串
		}

		// 将消息转换为 JSON
		response, err := json.Marshal(message)
		if err != nil {
			log.Println("生成用户列表 JSON 失败:", err)
			continue
		}

		// 向当前客户端发送用户列表
		err = client.Conn.WriteMessage(websocket.TextMessage, response)
		if err != nil {
			log.Printf("向客户端 %s 发送用户列表失败: %v\n", client.ID, err)
		}
	}
}

func broadNormalMessage(data string, targetClient *Client) {
	clientsMutex.Lock()
	defer clientsMutex.Unlock()

	// 判断是否是广播还是单发
	if targetClient == nil {
		// 创建消息结构
		message := Message{
			Sender:   "server",
			Receiver: "",
			Command:  "brodcastMes",
			Data:     data,
		}

		for _, client := range clients {
			message.Receiver = client.ID
			// 将消息转换为 JSON
			response, err := json.Marshal(message)
			if err != nil {
				log.Println("生成普通消息 JSON 失败:", err)
				return
			}
			err = client.Conn.WriteMessage(websocket.TextMessage, response)
			if err != nil {
				log.Printf("向客户端 %s 发送消息失败: %v\n", client.ID, err)
			}
		}
	} else {
		// 创建消息结构
		message := Message{
			Sender:   "server",
			Receiver: targetClient.ID,
			Command:  "brodcastMes",
			Data:     data,
		}

		// 将消息转换为 JSON
		response, err := json.Marshal(message)
		if err != nil {
			log.Println("生成普通消息 JSON 失败:", err)
			return
		}
		err = targetClient.Conn.WriteMessage(websocket.TextMessage, response)
		if err != nil {
			log.Printf("向客户端 %s 发送消息失败: %v\n", targetClient.ID, err)
		}
	}
}
