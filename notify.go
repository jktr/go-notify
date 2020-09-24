package notify

import (
	"errors"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	dbusRemoveMatch            = "org.freedesktop.DBus.RemoveMatch"
	dbusAddMatch               = "org.freedesktop.DBus.AddMatch"
	dbusObjectPath             = "/org/freedesktop/Notifications" // the DBUS object path
	dbusNotificationsInterface = "org.freedesktop.Notifications"  // DBUS Interface
	signalNotificationClosed   = "org.freedesktop.Notifications.NotificationClosed"
	signalActionInvoked        = "org.freedesktop.Notifications.ActionInvoked"
	callGetCapabilities        = "org.freedesktop.Notifications.GetCapabilities"
	callCloseNotification      = "org.freedesktop.Notifications.CloseNotification"
	callNotify                 = "org.freedesktop.Notifications.Notify"
	callGetServerInformation   = "org.freedesktop.Notifications.GetServerInformation"

	channelBufferSize = 10
)

// A Notification ID, to be used as a handle-like type.
type ID uint32

// Spec: Table 6. Notify Parameters
type Notification struct {
	// May be displayed to the user.
	AppName string
	// Spec: http://standards.freedesktop.org/icon-naming-spec/icon-naming-spec-latest.html
	AppIcon string

	// Setting ReplacesID atomically replaces another notification with this ID.
	ReplacesID ID

	Summary string
	// Some clients diplay the whole body in addition to,
	// or instead of, the summary (if not empty).
	Body string

	// A user may invoke these on a Notification.
	// Use Notifier to subscribe to these events.
	Actions []NotificationAction

	// Extension Mechanism for Notification Metadata.
	// Spec: https://specifications.freedesktop.org/notification-spec/latest/ar01s08.html
	Hints map[string]dbus.Variant

	// Strategy for notification expiration
	Expire Expiry
	// Timeout for the eponymous NotificationExpiry Strategy
	Timeout time.Duration
}

// NotificationAction represents a possible reaction to a notification
// This isn't a map[Name]Summary, as ordering of actions may be relevant.
type NotificationAction struct {
	// Identifier for this action.
	Name string
	// String displayed to user.
	Summary string
}

// Expiry specifies the policy for time-based notification auto-expiry.
type Expiry int32

const (
	// Uses notification server's default expiry behaviour.
	Server Expiry = iota - 1
	// Uses the Timeout from in Notification.
	Timeout
	// Never exire this notification automatically.
	Never
)

// Spec: https://specifications.freedesktop.org/notification-spec/latest/ar01s07.html
type Urgency byte

const (
	Low Urgency = iota
	Normal
	Critical
)

// Convenience function to add the urgency hint to a Notification.
func (note *Notification) SetUrgency(urgency Urgency) *Notification {
	if note.Hints == nil {
		note.Hints = make(map[string]dbus.Variant)
	}
	note.Hints["urgency"] = dbus.MakeVariant(urgency)
	return note
}

// See Notifier.Send
// This is provided for convenience; use Notifier if
// you'd like notification close events or user actions.
// Spec: org.freedesktop.Notifications.Notify
func Send(conn *dbus.Conn, note *Notification) (ID, error) {
	actions := []string{}
	if note.Actions != nil {
		actions = make([]string, 0, len(note.Actions)*2)
		for _, act := range note.Actions {
			actions = append(actions, act.Name)
			actions = append(actions, act.Summary)
		}
	}

	expire := int32(note.Expire)
	if expire > 0 {
		expire = int32(note.Timeout.Milliseconds())
	}

	obj := conn.Object(dbusNotificationsInterface, dbusObjectPath)
	call := obj.Call(callNotify, 0,
		note.AppName,
		note.ReplacesID,
		note.AppIcon,
		note.Summary,
		note.Body,
		actions,
		note.Hints,
		expire)

	if call.Err != nil {
		return 0, call.Err
	}

	var ret ID
	err := call.Store(&ret)
	return ret, err
}

// see Notifier.Dismiss
// Spec: org.freedesktop.Notifications.CloseNotification
func Dismiss(conn *dbus.Conn, id ID) error {
	if id == 0 {
		return errors.New("notification IDs must be greater than zero")
	}

	obj := conn.Object(dbusNotificationsInterface, dbusObjectPath)
	call := obj.Call(callCloseNotification, 0, id)
	return call.Err
}

// Spec: Table 7. GetServerInformation Return Values
type ServerInfo struct {
	Name        string
	Vendor      string
	Version     string
	SpecVersion string
}

// see Notifier.GetServerInfo
// Spec: org.freedesktop.Notifications.GetServerInformation
func GetServerInfo(conn *dbus.Conn) (*ServerInfo, error) {
	obj := conn.Object(dbusNotificationsInterface, dbusObjectPath)
	if obj == nil {
		return nil, errors.New("error creating dbus call object")
	}

	call := obj.Call(callGetServerInformation, 0)
	if call.Err != nil {
		return nil, call.Err
	}

	ret := ServerInfo{}
	err := call.Store(&ret.Name, &ret.Vendor, &ret.Version, &ret.SpecVersion)
	return &ret, err
}

// see Notifier.GetServerCapabilities
// Spec: org.freedesktop.Notifications.GetCapabilities
func GetServerCapabilities(conn *dbus.Conn) ([]string, error) {
	obj := conn.Object(dbusNotificationsInterface, dbusObjectPath)
	call := obj.Call(callGetCapabilities, 0)
	if call.Err != nil {
		return nil, call.Err
	}

	var ret []string
	err := call.Store(&ret)
	return ret, err
}
