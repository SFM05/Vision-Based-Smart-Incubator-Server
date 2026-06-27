package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"mqtt_listener/utils"

	MQTT "github.com/eclipse/paho.mqtt.golang"
)

var messageHandler MQTT.MessageHandler = func(client MQTT.Client, msg MQTT.Message) {
	// topic结构：device/$uuid/$request
	topic := strings.Split(string(msg.Topic()), "/")
	if len(topic) < 3 {
		slog.Error(fmt.Sprintf("Invalid topic encuntered: %s", msg.Topic()))
	}
	uuid := topic[1]
	request := topic[2]
	payload := string(msg.Payload())

	switch request {
	case "upload":
		slog.Info(fmt.Sprintf("Receive upload request from %s", uuid))
		utils.OnUploadRequest(client, uuid, payload)

	case "data":
		slog.Info(fmt.Sprintf("Receive environment data from %s", uuid))
		go utils.OnDataReceived(uuid, payload)

	default:
		slog.Warn(fmt.Sprintf("Receive unkown message from %s: %s", uuid, payload))
	}
}

func main() {
	username := os.Getenv("USERNAME")
	password := os.Getenv("PASSWORD")

	opts := MQTT.NewClientOptions()
	opts.AddBroker("tcp://localhost:1883")
	opts.SetClientID("go-subscriber-1")
	opts.SetUsername(username)
	opts.SetPassword(password)
	opts.SetDefaultPublishHandler(messageHandler)
	opts.SetAutoReconnect(true)
	opts.SetKeepAlive(60 * time.Second)

	client := MQTT.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		slog.Error(fmt.Sprintf("MQTT connect failed: %v", token.Error()))
	}
	slog.Info("MQTT connection established.")

	topic := "device/#"
	qos := 1
	if token := client.Subscribe(topic, byte(qos), nil); token.Wait() && token.Error() != nil {
		slog.Error(fmt.Sprintf("Subscribe topic failed: %v", token.Error()))
	}
	slog.Info(fmt.Sprintf("Subscribe topic success: %s(QoS %d)", topic, qos))

	// 退出信号 (Ctrl+C)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	fmt.Println("\nExiting...")
	client.Disconnect(250) // 等待250ms让Broker处理完断连
	fmt.Println("Exited")
}
