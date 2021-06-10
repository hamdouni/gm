package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"strings"

	"google.golang.org/api/gmail/v1"
)

func main() {
	log.Println("Contacting Gmail...")
	ctx := context.Background()
	srv, err := getService(ctx, "credentials.json")
	if err != nil {
		log.Fatalf("Unable to get Gmail Service: %v", err)
	}

	type message struct {
		size    int64
		gmailID string
		snippet string
		body    string
		// retrieved from message header
		date    string
		subject string
		from    string
		to      string
	}
	msgs := []message{}
	user := "me"

	log.Println("Retreive threads...")
	threads, err := srv.Users.Threads.List(user).Q("label:INBOX").Do()
	if err != nil {
		log.Fatalf("Unable to retrieve messages: %v", err)
	}
	if len(threads.Threads) == 0 {
		fmt.Println("No messages found.")
		return
	}
	log.Printf("Found %v threads.\n", len(threads.Threads))
	for idx, t := range threads.Threads {
		log.Printf("Retreiving messages in thread %v on %v.\n", idx+1, len(threads.Threads))
		thread, err := srv.Users.Threads.Get(user, t.Id).Do()
		if err != nil {
			log.Printf("Unable to retrieve thread: %v", err)
			continue
		}
		// get the latest message in the thread
		msg := thread.Messages[len(thread.Messages)-1]
		date, subject, from, to := "", "", "", ""
		for _, h := range msg.Payload.Headers {
			switch h.Name {
			case "Date":
				date = h.Value
			case "Subject":
				subject = h.Value
			case "From":
				from = h.Value
			case "To":
				to = h.Value
			default:
			}
		}

		data, err := base64.URLEncoding.DecodeString(msg.Payload.Body.Data)
		if err != nil {
			log.Fatalf("err: %v", err)
		}
		body := string(data)
		if len(body) == 0 {
			for _, b := range msg.Payload.Parts {
				if b.MimeType == "text/plain" {
					data, err := base64.URLEncoding.DecodeString(b.Body.Data)
					if err != nil {
						log.Fatalf("data: %v\nlen: %v\nerr: %v", b.Body.Data, len(b.Body.Data), err)
					}
					if len(data) == 0 {
						continue
					}
					body = string(data)
				}
			}
		}
		msgs = append(msgs, message{
			size:    msg.SizeEstimate,
			gmailID: msg.Id,
			snippet: msg.Snippet,
			body:    body,
			date:    date,
			subject: subject,
			from:    from,
			to:      to,
		})
	}
	log.Println("Ready.")
	reader := bufio.NewReader(os.Stdin)
	count, deleted, archived := 0, 0, 0
	for _, m := range msgs {
		count++
		for {
			fmt.Println("\n--------------------------------------------------------------------------------")
			fmt.Printf("Subject: %s\nFrom: %s\nTo: %s\nSize: %v\nDate: %v\n\n", m.subject, m.from, m.to, m.size, m.date)
			fmt.Printf("Options: (v)iew, (a)rchive, (o)pen, (d)elete, (s)kip, (q)uit: [s] ")
			skip := false
			val := ""
			if val, err = reader.ReadString('\n'); err != nil {
				log.Fatalf("Unable to read input: %v", err)
			}
			val = strings.TrimSpace(val)
			switch val {
			case "o": // open in browser
				openbrowser("https://mail.google.com/mail/u/0/#all/" + m.gmailID)
			case "v": // view message body
				fmt.Printf("\033c%s\n", m.body)
			case "d": // delete message
				if err := srv.Users.Messages.Delete("me", m.gmailID).Do(); err != nil {
					log.Fatalf("Unable to delete message %v: %v", m.gmailID, err)
				}
				log.Printf("Deleted message %v.\n", m.gmailID)
				deleted++
				skip = true
			case "a": // archive message by removing 'inbox' label
				_, err := srv.Users.Messages.Modify("me", m.gmailID, &gmail.ModifyMessageRequest{
					RemoveLabelIds: []string{"INBOX"},
				}).Do()
				if err != nil {
					log.Fatalf("Unable to archive message %v: %v", m.gmailID, err)
				}
				log.Printf("Archived message %v.\n", m.gmailID)
				archived++
				skip = true
			case "q": // quit
				log.Printf("Done.  %v messages processed, %v deleted, %v archived\n", count, deleted, archived)
				os.Exit(0)
			default:
				skip = true
			}
			if skip {
				break
			}
		}
	}
}
