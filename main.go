package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"strings"

	"github.com/joho/godotenv"
	"github.com/m1guelpf/chatgpt-telegram/src/chatgpt"
	"github.com/m1guelpf/chatgpt-telegram/src/config"
	"github.com/m1guelpf/chatgpt-telegram/src/session"
	"github.com/m1guelpf/chatgpt-telegram/src/tgbot"
)

func main() {
	config, err := config.Init()
	if err != nil {
		log.Fatalf("Couldn't load config: %v", err)
	}

	if config.OpenAISession == "" {
		session, err := session.GetSession()
		if err != nil {
			log.Fatalf("Couldn't get OpenAI session: %v", err)
		}

		err = config.Set("OpenAISession", session)
		if err != nil {
			log.Fatalf("Couldn't save OpenAI session: %v", err)
		}
	}

	chatGPT := chatgpt.Init(config)
	log.Println("Started ChatGPT")

	err = godotenv.Load()
	if err != nil {
		log.Fatalf("Couldn't load .env file: %v", err)
	}

	bot, err := tgbot.New(os.Getenv("TELEGRAM_TOKEN"))
	if err != nil {
		log.Fatalf("Couldn't start Telegram bot: %v", err)
	}

	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		bot.Stop()
		os.Exit(0)
	}()

	log.Printf("Started Telegram bot! Message @%s to start.", bot.Username)

	for update := range bot.GetUpdatesChan() {
		if update.Message == nil {
			continue
		}

		var (
			updateText      = update.Message.Text
			updateChatID    = update.Message.Chat.ID
			updateMessageID = update.Message.MessageID
		)

		//Check TELEGRAM_ID only if Chat Type is Private.
		//This Admit Group Chat. In @BotFather you can change if bot dissable or enable Groups.  
		userId := strconv.FormatInt(update.Message.Chat.ID, 10)
		if os.Getenv("TELEGRAM_ID") != "" && userId != os.Getenv("TELEGRAM_ID") && update.Message.Chat.Type == "private" {
			bot.Send(updateChatID, updateMessageID, "You are not authorized to use this bot.")
			continue
		}

		// Add Support to Group and SuperGroups for Bots in Private Mode.
		// The Telegram Bots is setup by default in Private Mode, only receive commands "/command".
		// 'askgpt' Command is used in Group for bot receive Text.
		if !update.Message.IsCommand() || update.Message.Command() == "askgpt" {
			bot.SendTyping(updateChatID)

			//Remove Command from updateText and Empty Space.
			if update.Message.IsCommand(){
				splitText := strings.Split(updateText, " ")
				updateText = strings.Join(splitText[1:], " ")
				updateText = strings.Trim(updateText, " ")
			}  
			//Press Commands in Telegram Menu send Empty String, this prevents it.
			if len(updateText) == 0 {
				bot.Send(updateChatID, updateMessageID, "Empty text, must write something")
				continue
			}
			feed, err := chatGPT.SendMessage(updateText, updateChatID)
			if err != nil {
				bot.Send(updateChatID, updateMessageID, fmt.Sprintf("Error: %v", err))
			} else {
				bot.SendAsLiveOutput(updateChatID, updateMessageID, feed)
			}
			continue
		}

		var text string
		switch update.Message.Command() {
		case "help":
			text = "Send a message to start talking with ChatGPT. If you are in a group use /askgpt + yourMessage. You can use /reload at any point to clear the conversation history and start from scratch (don't worry, it won't delete the Telegram messages)."
		case "start":
			text = "Send a message to start talking with ChatGPT. If you are in a group use /askgpt + yourMessage. You can use /reload at any point to clear the conversation history and start from scratch (don't worry, it won't delete the Telegram messages)."
		case "reload":
			chatGPT.ResetConversation(updateChatID)
			text = "Started a new conversation. Enjoy!"
		default:
			text = "Unknown command. Send /help to see a list of commands."
		}

		if _, err := bot.Send(updateChatID, updateMessageID, text); err != nil {
			log.Printf("Error sending message: %v", err)
		}
	}
}
