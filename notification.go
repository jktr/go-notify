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

// ServerInfo is a holder for information returned by
// GetServerInformation call.
type ServerInfo struct {
	Name        string
	Vendor      string
	Version     string
	SpecVersion string
}

// GetServerInfo returns the information on the server.
//
// org.freedesktop.Notifications.GetServerInformation
//
//  GetServerInformation Return Values
//
//		Name		 Type	  Description
//		name		 STRING	  The product name of the server.
//		vendor		 STRING	  The vendor name. For example, "KDE," "GNOME," "freedesktop.org," or "Microsoft."
//		version		 STRING	  The server's version number.
//		spec_version STRING	  The specification version the server is compliant with.
//
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

// GetCapabilities gets the capabilities of the notification server.
// This call takes no parameters.
// It returns an array of strings. Each string describes an optional capability implemented by the server.
//
// See also: https://developer.gnome.org/notification-spec/
// GetCapabilities provide an exported method for this operation
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

// Notifier is an interface implementing the operations supported by the
// Freedesktop DBus Notifications object.
//
// New() sets up a Notifier that listens on dbus' signals regarding
// Notifications: NotificationClosed and ActionInvoked.
//
// Signal delivery works by subscribing to all signals regarding Notifications,
// which means you will see signals for Notifications also from other sources,
// not just the latest you sent
//
// Users that only want to send a simple notification, but don't care about
// interacting with signals, can use exported method: SendNotification(conn, Notification)
//
// Caller is responsible for calling Close() before exiting,
// to shut down event loop and cleanup dbus registration.
type Notifier interface {
	Send(n *Notification) (ID, error)
	Dismiss(id ID) error
	GetServerCapabilities() ([]string, error)
	GetServerInfo() (*ServerInfo, error)
	Close() error
}

// NotificationClosedHandler is called when we receive a NotificationClosed signal
type NotificationClosedHandler func(id ID, reason CloseReason)

// ActionInvokedHandler is called when we receive a signal that one of the action_keys was invoked.
//
// Note that invoking an action often also produces a NotificationClosedSignal,
// so you might receive both a Closed signal and a ActionInvoked signal.
//
// I suspect this detail is implementation specific for the UI interaction,
// and does at least happen on XFCE4.
type ActionInvokedHandler func(id ID, actionName string)

// notifier implements Notifier interface
type notifier struct {
	conn     *dbus.Conn
	signal   chan *dbus.Signal
	onClosed NotificationClosedHandler
	onAction ActionInvokedHandler
}

// option overrides certain parts of a Notifier
type option func(*notifier)

// WithOnAction sets ActionInvokedHandler handler
func WithOnAction(h ActionInvokedHandler) option {
	return func(n *notifier) {
		n.onAction = h
	}
}

// WithOnClosed sets NotificationClosed handler
func WithOnClosed(h NotificationClosedHandler) option {
	return func(n *notifier) {
		n.onClosed = h
	}
}

// New creates a new Notifier using conn.
// See also: Notifier
func New(conn *dbus.Conn, opts ...option) (Notifier, error) {
	n := &notifier{
		conn:   conn,
		signal: make(chan *dbus.Signal, channelBufferSize),
	}

	for _, val := range opts {
		val(n)
	}

	// add a listener (matcher) in dbus for signals to Notification interface.
	if err := n.conn.AddMatchSignal(
		dbus.WithMatchObjectPath(dbusObjectPath),
		dbus.WithMatchInterface(dbusNotificationsInterface),
	); err != nil {
		return nil, err
	}

	// register in dbus for signal delivery
	n.conn.Signal(n.signal)

	// start eventloop
	go n.receiveSignals()

	return n, nil
}

func (n *notifier) receiveSignals() {
	for {
		select {
		case <-n.conn.Context().Done():
			return
		case signal := <-n.signal:
			switch signal.Name {
			case signalNotificationClosed:
				if n.onClosed != nil {
					go n.onClosed(
						signal.Body[0].(ID),
						CloseReason(signal.Body[1].(uint32)),
					)
				}
			case signalActionInvoked:
				if n.onAction != nil {
					go n.onAction(
						signal.Body[0].(ID),
						signal.Body[1].(string),
					)
				}
			}
		}
	}
}

func (n *notifier) GetServerCapabilities() ([]string, error) {
	return GetServerCapabilities(n.conn)
}
func (n *notifier) GetServerInfo() (*ServerInfo, error) {
	return GetServerInfo(n.conn)
}

// Send sends a notification to the notification server and returns the ID or an error.
//
// Implements dbus call:
//
//     UINT32 org.freedesktop.Notifications.Notify (
//	       STRING app_name,
//	       UINT32 replaces_id,
//	       STRING app_icon,
//	       STRING summary,
//	       STRING body,
//	       ARRAY  actions,
//	       DICT   hints,
//	       INT32  expire_timeout
//     );
//
//		Name	    	Type	Description
//		app_name		STRING	The optional name of the application sending the notification. Can be blank.
//		replaces_id	    UINT32	The optional notification ID that this notification replaces. The server must atomically (ie with no flicker or other visual cues) replace the given notification with this one. This allows clients to effectively modify the notification while it's active. A value of value of 0 means that this notification won't replace any existing notifications.
//		app_icon		STRING	The optional program icon of the calling application. Can be an empty string, indicating no icon.
//		summary		    STRING	The summary text briefly describing the notification.
//		body			STRING	The optional detailed body text. Can be empty.
//		actions		    ARRAY	Actions are sent over as a list of pairs. Each even element in the list (starting at index 0) represents the identifier for the action. Each odd element in the list is the localized string that will be displayed to the user.
//		hints	        DICT	Optional hints that can be passed to the server from the client program. Although clients and servers should never assume each other supports any specific hints, they can be used to pass along information, such as the process PID or window ID, that the server may be able to make use of. See Hints. Can be empty.
//      expire_timeout  INT32   The timeout time in milliseconds since the display of the notification at which the notification should automatically close.
//								If -1, the notification's expiration time is dependent on the notification server's settings, and may vary for the type of notification. If 0, never expire.
//
// If replaces_id is 0, the return value is a UINT32 that represent the notification.
// It is unique, and will not be reused unless a MAXINT number of notifications have been generated.
// An acceptable implementation may just use an incrementing counter for the ID.
// The returned ID is always greater than zero. Servers must make sure not to return zero as an ID.
//
// If replaces_id is not 0, the returned value is the same value as replaces_id.
func (n *notifier) Send(note *Notification) (ID, error) {
	return Send(n.conn, note)
}

// Dismiss causes a notification to be forcefully closed and removed from the user's view.
// It can be used, for example, in the event that what the notification pertains to is no longer relevant,
// or to cancel a notification with no expiration time.
//
// The NotificationClosed (dbus) signal is emitted by this method.
// If the notification no longer exists, an empty D-BUS Error message is sent back.
func (n *notifier) Dismiss(id ID) error {
	obj := n.conn.Object(dbusNotificationsInterface, dbusObjectPath)
	call := obj.Call(callCloseNotification, 0, id)
	return call.Err
}

// Reason for the closed notification
type CloseReason uint32

const (
	// Expired when a notification expired
	Expired CloseReason = iota + 1

	// DismissedByUser when a notification has been dismissed by a user
	DismissedByUser

	// ClosedByCall when a notification has been closed by a call to CloseNotification
	DismissedByCall

	// Unknown when as notification has been closed for an unknown reason
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

// Close cleans up and shuts down signal delivery loop
func (n *notifier) Close() error {
	// remove signal reception
	n.conn.RemoveSignal(n.signal)

	// unregister in dbus
	return n.conn.RemoveMatchSignal(
		dbus.WithMatchObjectPath(dbusObjectPath),
		dbus.WithMatchInterface(dbusNotificationsInterface),
	)
}
