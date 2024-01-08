package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	notekeeper "matthewrv/note-keeper"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
)

type scheduledMessage struct {
	ChatID    int64
	MessageID int
	Created   time.Time
	NextAt    time.Time
	Complete  bool
}

func (msg *scheduledMessage) shouldBeSent() bool {
	return msg.NextAt.Before(time.Now())
}

func (msg *scheduledMessage) updateNextAt(intervals []time.Duration) {
	isNextAtSet := false

	for _, interval := range intervals {
		tmp := msg.Created.Add(interval)
		if tmp.After(time.Now()) {
			msg.NextAt = tmp
			isNextAtSet = true
			break
		}
	}

	msg.Complete = !isNextAtSet
}

type scheduledMessagesChannel chan scheduledMessage

type Config struct {
	bot          *tgbotapi.BotAPI
	intervals    []time.Duration
	allowedChats []int64
}

func processUpdates(ctx context.Context, config *Config, messagesToSchedule scheduledMessagesChannel) {
	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 30
	updates := config.bot.GetUpdatesChan(updateConfig)
	for {
		select {
		// stop looping if ctx is cancelled
		case <-ctx.Done():
			return
		// receive update from channel and then handle it
		case update := <-updates:
			handleUpdate(config, update, messagesToSchedule)
		}
	}
}

func handleUpdate(config *Config, update tgbotapi.Update, messagesToSchedule scheduledMessagesChannel) {
	if update.Message == nil {
		return
	}

	chatId := update.Message.Chat.ID
	messageId := update.Message.MessageID
	log.Printf("%d-%d: Start handling update", chatId, messageId)

	var msg = tgbotapi.NewMessage(update.Message.Chat.ID, "")

	if !slices.Contains[[]int64, int64](config.allowedChats, chatId) {
		msg.Text = "Sorry, you are not allowed to use this bot 😢"
		log.Printf("%d-%d: User not authorized to use bot", chatId, messageId)
	} else if update.Message.Text == "" {
		msg.Text = "Nothing to remind you about."
		log.Printf("%d-%d: Empty message", chatId, messageId)
	} else {
		msg.ReplyToMessageID = update.Message.MessageID

		_, err := notekeeper.SaveNote(update.Message.Text)
		if err != nil {
			msg.Text = "Failed to save note 😢 Try sending message again. If it does not help - we f*cked up."
			log.Printf("%d-%d: Failed to save note", chatId, messageId)
		} else {
			msg.Text = "Message saved! We will remind you about it 😉"

			// push new entry to reminders goroutine
			now := time.Now()
			nextAt := now.Add(config.intervals[0])
			messagesToSchedule <- scheduledMessage{update.Message.Chat.ID, update.Message.MessageID, now, nextAt, false}
			log.Printf("%d-%d: Note saved successfully", chatId, messageId)
		}
	}

	if _, err := config.bot.Send(msg); err != nil {
		log.Printf("Error sending response to chat %d: %s", update.Message.Chat.ID, err)
	}
}

func insert(ss []scheduledMessage, s scheduledMessage) []scheduledMessage {
	i := sort.Search(len(ss), func(i int) bool { return s.NextAt.After(ss[i].NextAt) })
	ss = append(ss, scheduledMessage{})
	copy(ss[i+1:], ss[i:])
	ss[i] = s
	return ss
}

func scheduleReminders(ctx context.Context, config *Config, ticker *time.Ticker, messagesToSchedule scheduledMessagesChannel) {

	var allReminders []scheduledMessage
	for {
		select {
		// stop looping if ctx is cancelled
		case <-ctx.Done():
			return
		// receive update from channel and then handle it
		case scheduledMsg := <-messagesToSchedule:
			allReminders = insert(allReminders, scheduledMsg)
		case <-ticker.C:
			// peek all reminders
			log.Print("Period has passed, scheduling messages to send")
			sentMessagesCount := 0
			for i := range allReminders {
				current := &allReminders[i]
				if !current.shouldBeSent() {
					break
				}
				sendReminder(config, current)
				sentMessagesCount = i
			}
			if sentMessagesCount > 0 {
				slices.SortFunc(allReminders, func(a scheduledMessage, b scheduledMessage) int { return int(a.NextAt.Sub(b.NextAt).Nanoseconds()) })
			}
		}
	}
}

func sendReminder(config *Config, msg *scheduledMessage) {
	passed := time.Since(msg.Created)
	text := fmt.Sprintf("It is time to remind you about this. Time passed since creation: %s", passed)
	tgMsg := tgbotapi.NewMessage(msg.ChatID, text)
	tgMsg.ReplyToMessageID = msg.MessageID

	if _, err := config.bot.Send(tgMsg); err != nil {
		log.Printf(
			"%d-%d: Failed to send notification: %s",
			msg.ChatID,
			msg.MessageID,
			err,
		)
		return
	}
	msg.updateNextAt(config.intervals)
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal(err)
	}

	config := Config{}
	config.intervals = []time.Duration{
		time.Duration(time.Hour * 24),
		time.Duration(time.Hour * 24 * 3),
		time.Duration(time.Hour * 24 * 10),
		time.Duration(time.Hour * 24 * 30),
	}

	allowedChatsString := strings.Split(os.Getenv("ALLOWED_CHATS"), ",")
	config.allowedChats = []int64{}
	for _, chatId := range allowedChatsString {
		parsedChatId, err := strconv.ParseInt(chatId, 10, 64)
		if err != nil {
			log.Fatalf("Invalid chat id %s - should be integer", chatId)
		}
		config.allowedChats = append(config.allowedChats, parsedChatId)
	}

	token := os.Getenv("TELEGRAM_APITOKEN")
	config.bot, err = tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Panic(err)
	}

	debug, err := strconv.ParseBool(os.Getenv("BOT_DEBUG_MODE"))
	if err != nil {
		log.Panic(err)
	}
	config.bot.Debug = debug

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	messagesToSchedule := make(chan scheduledMessage, 100)

	tickerPeriodString := os.Getenv("TICKER_PERIOD")
	if tickerPeriodString == "" {
		tickerPeriodString = "10s"
	}
	log.Printf("Ticker period would be set to %s", tickerPeriodString)

	period, err := time.ParseDuration(tickerPeriodString)
	if err != nil {
		log.Fatalf("Failed to parse message sent period: %s", err)
	}
	ticker := time.NewTicker(period)

	go processUpdates(ctx, &config, messagesToSchedule)
	go scheduleReminders(ctx, &config, ticker, messagesToSchedule)
	// TODO: load reminders from file system on start up

	log.Println("Start listening for updates. Press Ctrl+C to stop")

	// Wait for a newline symbol, then cancel handling updates
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
	cancel()
}
