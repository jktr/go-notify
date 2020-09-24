package main

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/jktr/go-notify"
)

func main() {

	wg := &sync.WaitGroup{}
	wg.Add(2)

	conn, err := dbus.SessionBusPrivate()
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	if err = conn.Auth(nil); err != nil {
		log.Fatal(err)
	}
	if err = conn.Hello(); err != nil {
		log.Fatal(err)
	}

	// prepare a Notification for sending ...
	n := &notify.Notification{
		AppName: "go-notify example app",
		AppIcon: "mail-unread",
		Summary: "go-notify example summary",
		Body:    "This is the body of an extended go-notify example.",
		Timeout: 5 * time.Second,
	}
	n.SetActions("confirm", "Confirm.", "cancel", "Cancel.")
	n.SetUrgency(notify.Critical)

	// ... and then show it
	createdID, err := notify.Send(conn, n)
	if err != nil {
		log.Fatal("error sending notification:", err)
	}
	log.Printf("created notification with id: %d", createdID)

	// list server features
	caps, err := notify.GetServerCapabilities(conn)
	if err != nil {
		log.Fatal("error fetching capabilities:", err)
	}
	for x := range caps {
		fmt.Printf("Registered capability: %s\n", caps[x])
	}

	// list server vendor metadata
	info, err := notify.GetServerInfo(conn)
	if err != nil {
		log.Fatal("error getting server information:", err)
	}
	fmt.Printf("Name:    %v\n", info.Name)
	fmt.Printf("Vendor:  %v\n", info.Vendor)
	fmt.Printf("Version: %v\n", info.Version)
	fmt.Printf("Spec:    %v\n", info.SpecVersion)

	onAction := func(id notify.ID, action string) {
		log.Printf("ActionInvoked: %d Key: %s", id, action)
	}
	onClosed := func(id notify.ID, reason notify.CloseReason) {
		log.Printf("NotificationClosed: %d Reason: %s", id, reason)
		wg.Done()
	}

	// Notifier interface with event delivery
	notifier, err := notify.New(
		conn,
		notify.WithOnAction(onAction),
		notify.WithOnClosed(onClosed),
	)

	if err != nil {
		log.Fatal(err)
	}
	defer notifier.Close()

	id, err := notifier.Send(n)
	if err != nil {
		log.Fatalf("error sending notification: %v", err)
	}
	log.Printf("sent notification id: %v", id)

	wg.Wait()
}
