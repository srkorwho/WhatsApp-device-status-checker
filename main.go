package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "modernc.org/sqlite"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
)

type TimingData struct {
	MessageID        string
	SentTime         time.Time
	ServerTime       time.Time
	DeliveredTime    time.Time
	ServerDuration   time.Duration
	DeliveryDuration time.Duration
	ReadTime         time.Time
}

type MessageTracking struct {
	SentTime   time.Time
	ServerTime time.Time
	HasServer  bool
}

type TimingBot struct {
	client      *whatsmeow.Client
	targetPhone types.JID
	interval    time.Duration
	timingData  []TimingData
	pendingMsgs map[string]*MessageTracking
}

func NewTimingBot(client *whatsmeow.Client, targetPhone types.JID, interval time.Duration) *TimingBot {
	return &TimingBot{
		client:      client,
		targetPhone: targetPhone,
		interval:    interval,
		timingData:  make([]TimingData, 0),
		pendingMsgs: make(map[string]*MessageTracking),
	}
}

func (tb *TimingBot) handleReceipt(receipt *events.Receipt) {
	for _, msgID := range receipt.MessageIDs {
		tracking, exists := tb.pendingMsgs[msgID]
		if !exists {
			continue
		}

		now := time.Now()

		if receipt.Type == events.ReceiptTypeDelivered && !tracking.HasServer {
			tracking.ServerTime = now
			tracking.HasServer = true
			serverDuration := now.Sub(tracking.SentTime)
			fmt.Printf("[1st tick] %d ms\n", serverDuration.Milliseconds())
			continue
		}

		if receipt.Type == events.ReceiptTypeDelivered && tracking.HasServer {
			deliveryDuration := now.Sub(tracking.ServerTime)
			serverDuration := tracking.ServerTime.Sub(tracking.SentTime)

			data := TimingData{
				MessageID:        msgID,
				SentTime:         tracking.SentTime,
				ServerTime:       tracking.ServerTime,
				DeliveredTime:    now,
				ServerDuration:   serverDuration,
				DeliveryDuration: deliveryDuration,
			}

			tb.timingData = append(tb.timingData, data)
			delete(tb.pendingMsgs, msgID)
			tb.printTiming(data)
		}

		if receipt.Type == events.ReceiptTypeRead {
			for i := range tb.timingData {
				if tb.timingData[i].MessageID == msgID {
					tb.timingData[i].ReadTime = now
					fmt.Printf("[read] %s\n", now.Format("15:04:05.000"))
					break
				}
			}
		}
	}
}

func (tb *TimingBot) printTiming(data TimingData) {
	fmt.Println("\n" + strings.Repeat("-", 60))
	fmt.Printf("msg id: %s\n", data.MessageID)
	fmt.Printf("sent:      %s\n", data.SentTime.Format("15:04:05.000"))
	fmt.Printf("server:    %s\n", data.ServerTime.Format("15:04:05.000"))
	fmt.Printf("delivered: %s\n", data.DeliveredTime.Format("15:04:05.000"))
	fmt.Printf("[1st -> 2nd tick]: %d ms\n", data.DeliveryDuration.Milliseconds())
	if !data.ReadTime.IsZero() {
		fmt.Printf("read:      %s\n", data.ReadTime.Format("15:04:05.000"))
	}
	fmt.Println(strings.Repeat("-", 60))
}

func (tb *TimingBot) sendTestMessage() error {
	msg := &waProto.Message{
		Conversation: proto.String("."),
	}

	sentTime := time.Now()
	resp, err := tb.client.SendMessage(context.Background(), tb.targetPhone, msg)
	if err != nil {
		return fmt.Errorf("send failed: %v", err)
	}

	tb.pendingMsgs[resp.ID] = &MessageTracking{
		SentTime:  sentTime,
		HasServer: false,
	}
	
	fmt.Printf("\n[sent] %s | id: %s\n", sentTime.Format("15:04:05.000"), resp.ID)
	return nil
}

func (tb *TimingBot) Start() {
	fmt.Printf("target: %s\n", tb.targetPhone)
	fmt.Printf("interval: %v\n\n", tb.interval)

	ticker := time.NewTicker(tb.interval)
	defer ticker.Stop()

	if err := tb.sendTestMessage(); err != nil {
		fmt.Printf("error: %v\n", err)
	}

	for range ticker.C {
		if err := tb.sendTestMessage(); err != nil {
			fmt.Printf("error: %v\n", err)
		}
	}
}

func (tb *TimingBot) ShowStatistics() {
	if len(tb.timingData) == 0 {
		return
	}

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("STATS")
	fmt.Println(strings.Repeat("=", 60))

	var totalDuration time.Duration
	minDuration := tb.timingData[0].DeliveryDuration
	maxDuration := tb.timingData[0].DeliveryDuration

	for _, data := range tb.timingData {
		totalDuration += data.DeliveryDuration
		if data.DeliveryDuration < minDuration {
			minDuration = data.DeliveryDuration
		}
		if data.DeliveryDuration > maxDuration {
			maxDuration = data.DeliveryDuration
		}
	}

	avgDuration := totalDuration / time.Duration(len(tb.timingData))

	fmt.Printf("total: %d\n", len(tb.timingData))
	fmt.Printf("min: %d ms\n", minDuration.Milliseconds())
	fmt.Printf("max: %d ms\n", maxDuration.Milliseconds())
	fmt.Printf("avg: %d ms\n", avgDuration.Milliseconds())
	fmt.Println(strings.Repeat("=", 60))
}

var bot *TimingBot

func main() {
	dbFile := "session.db"
	defer os.Remove(dbFile)
	
	dbLog := waLog.Stdout("Database", "ERROR", true)
	container, err := sqlstore.New(context.Background(), "sqlite", "file:"+dbFile+"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_busy_timeout(5000)", dbLog)
	if err != nil {
		panic(err)
	}

	deviceStore, err := container.GetFirstDevice(context.Background())
	if err != nil {
		panic(err)
	}

	clientLog := waLog.Stdout("Client", "ERROR", true)
	client := whatsmeow.NewClient(deviceStore, clientLog)
	client.EnableAutoReconnect = true

	client.AddEventHandler(func(evt interface{}) {
		switch v := evt.(type) {
		case *events.Receipt:
			if bot != nil {
				bot.handleReceipt(v)
			}
		}
	})

	qrChan, _ := client.GetQRChannel(context.Background())
	err = client.Connect()
	if err != nil {
		panic(err)
	}

	fmt.Println("scan qr code:")
	fmt.Println(strings.Repeat("=", 60))
	for evt := range qrChan {
		if evt.Event == "code" {
			fmt.Println("\n" + evt.Code)
		} else if evt.Event == "success" {
			fmt.Println("\nconnected\n")
			break
		}
	}

	var targetPhoneStr string
	var intervalSeconds int

	fmt.Print("target number (e.g. 905551234567): ")
	fmt.Scanln(&targetPhoneStr)

	if !strings.Contains(targetPhoneStr, "@") {
		targetPhoneStr = targetPhoneStr + "@s.whatsapp.net"
	}
	
	targetPhone, err := types.ParseJID(targetPhoneStr)
	if err != nil {
		panic(fmt.Sprintf("invalid number: %v", err))
	}

	fmt.Print("interval (seconds): ")
	fmt.Scanln(&intervalSeconds)

	interval := time.Duration(intervalSeconds) * time.Second

	bot = NewTimingBot(client, targetPhone, interval)

	go bot.Start()

	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			bot.ShowStatistics()
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	fmt.Println("\n")
	bot.ShowStatistics()

	client.Disconnect()
	fmt.Println("stopped")
}
