package notify

import (
	"context"

	"github.com/godbus/dbus/v5"
)

// Notifier is an interface implementing the operations
// supported by the Freedesktop DBus Notifications object.
//
// In contrast to the top-level convenience methods, this
// type listens to notification action and close events events.
// These are delivered to handlers, which are each invoked in a
// fresh gouroutine.
//
// Note that Signal delivery currently works by subscribing to
// all signals, only filtering on signal type. You will see
// signals for Notifications that other sources have sent.
// Use Send's return value to filter for relevant Notifications.
//
// Notifier.Close() should be called before shutting down
// the underlying connection to ensure a clean shutdown.
type Notifier interface {
	Send(n *Notification) (ID, error)
	Dismiss(id ID) error
	GetServerCapabilities() ([]string, error)
	GetServerInfo() (*ServerInfo, error)
	Close() error
}

// Reason for notification close on server side.
// Spec: Table 8. NotificationClosed Parameters
type CloseReason uint32

const (
	// Notifcation reached a Timeout and expired
	Expired CloseReason = iota + 1

	// A user dismissed the notification
	DismissedByUser

	// caused by org.freedesktop.Notifications.CloseNotification
	DismissedByCall

	// Undefined or Reserved reasons
	Unknown
)

func (r CloseReason) String() string {
	switch r {
	case Expired:
		return "Expired"
	case DismissedByUser:
		return "DismissedByUser"
	case DismissedByCall:
		return "ClosedByCall"
	case Unknown:
		return "Unknown"
	default:
		return "Other"
	}
}

// Called on receipt of a notification close event
// Spec: org.freedesktop.Notifications.NotificationClosed
type NotificationClosedHandler func(id ID, reason CloseReason)

// Called on receipt of a notification action invokation.
//
// Note that many server implementations dismiss notifications
// immediately before/after/during invocation of an action.
// As a result, you may see Close and Action events about
// the same notification in close temporal proximity.
//
// Spec: org.freedesktop.Notifications.ActionInvoked
type ActionInvokedHandler func(id ID, actionName string)

// implements Notifier
type notifier struct {
	conn     *dbus.Conn
	signal   chan *dbus.Signal
	ctx      context.Context
	shutdown context.CancelFunc
	onClosed NotificationClosedHandler
	onAction ActionInvokedHandler
}

// functional configuration type
type option func(*notifier)

func WithOnAction(h ActionInvokedHandler) option {
	return func(n *notifier) {
		n.onAction = h
	}
}

func WithOnClosed(h NotificationClosedHandler) option {
	return func(n *notifier) {
		n.onClosed = h
	}
}

func New(conn *dbus.Conn, opts ...option) (Notifier, error) {
	ctx, cancel := context.WithCancel(conn.Context())

	n := &notifier{
		conn:     conn,
		signal:   make(chan *dbus.Signal, channelBufferSize),
		ctx:      ctx,
		shutdown: cancel,
	}

	for _, val := range opts {
		val(n)
	}

	// subscribe to notification signals
	if err := n.conn.AddMatchSignal(
		dbus.WithMatchObjectPath(dbusObjectPath),
		dbus.WithMatchInterface(dbusNotificationsInterface),
	); err != nil {
		return nil, err
	}
	n.conn.Signal(n.signal)

	go n.receiveSignals()

	return n, nil
}

func (n *notifier) receiveSignals() {
	for {
		select {
		case <-n.ctx.Done():
			return
		case signal := <-n.signal:
			switch signal.Name {
			case signalNotificationClosed:
				if n.onClosed != nil {
					go n.onClosed(
						ID(signal.Body[0].(uint32)),
						CloseReason(signal.Body[1].(uint32)),
					)
				}
			case signalActionInvoked:
				if n.onAction != nil {
					go n.onAction(
						ID(signal.Body[0].(uint32)),
						signal.Body[1].(string),
					)
				}
			}
		}
	}
}

// Release Subscriptions to Notification Events
func (n *notifier) Close() error {
	n.shutdown()

	// unsubscribe
	n.conn.RemoveSignal(n.signal)
	return n.conn.RemoveMatchSignal(
		dbus.WithMatchObjectPath(dbusObjectPath),
		dbus.WithMatchInterface(dbusNotificationsInterface),
	)
}

// Send sends a notification to the notification server.
// The returned ID can be used as a handle to dismiss the
// notification and filter for Close/Action events in handlers.
//
// Spec: org.freedesktop.Notifications.Notify
func (n *notifier) Send(note *Notification) (ID, error) {
	return Send(n.conn, note)
}

// Dismiss causes a notification to be forcefully closed
// and removed from the user's view. It can be used, for example,
// in the event that what the notification pertains to is no
// longer relevant, or to cancel a notification with no expiration time.
//
// Spec: org.freedesktop.Notifications.CloseNotification
func (n *notifier) Dismiss(id ID) error {
	return Dismiss(n.conn, id)
}

// Queries the notification server for the list
// of optional features it supports.
//
// Spec: org.freedesktop.Notifications.GetCapabilities
// See also: https://developer.gnome.org/notification-spec/
func (n *notifier) GetServerCapabilities() ([]string, error) {
	return GetServerCapabilities(n.conn)
}

// Queries the notification server for vendor, product,
// and version metadata. Consider using a comibination
// of Hints and Server Capabilities if you would like
// to negotiate additional features.
//
// Spec: org.freedesktop.Notifications.GetServerInformation
func (n *notifier) GetServerInfo() (*ServerInfo, error) {
	return GetServerInfo(n.conn)
}
