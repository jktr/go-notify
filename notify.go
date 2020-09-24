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

type ID uint32

// Notification holds all information needed for creating a notification
// The zero value for a Notification is legal notification, albeit
// not a particularly useful one. You should at least set a Summary.
type Notification struct {
	AppName string
	// Setting ReplacesID atomically replaces another notification with this ID.
	ReplacesID ID
	// See predefined icons here: http://standards.freedesktop.org/icon-naming-spec/icon-naming-spec-latest.html
	AppIcon string
	Summary string
	Body    string
	Actions []NotificationAction
	Hints   map[string]dbus.Variant

	// Strategy for notification expiration
	Expire NotificationExpiry
	// Timeout for the eponymous NotificationExpiry Strategy
	Timeout time.Duration
}

// NotificationAction represents an possible reaction to a notification
type NotificationAction struct {
	// Identifier for this action.
	Name string
	// String displayed to user.
	Summary string
}

type NotificationExpiry int32

const (
	// Uses server's default expiry behaviour.
	Server NotificationExpiry = iota - 1
	// Uses the Notification's Timeout duration.
	Timeout
	// Never exire this notification.
	Never
)

// SendNotification is provided for convenience.
// Use if you only want to deliver a notification and dont care about events.
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

func Dismiss(conn *dbus.Conn, id ID) error {
	if id == 0 {
		return errors.New("notification IDs must be greater than zero")
	}

	obj := conn.Object(dbusNotificationsInterface, dbusObjectPath)
	call := obj.Call(callCloseNotification, 0, id)
	return call.Err
}

// ServerInfo is a holder for information returned by
// GetServerInformation call.
type ServerInfo struct {
	Name        string
	Vendor      string
	Version     string
	SpecVersion string
}

// see Notifier.GetServerInfo
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
