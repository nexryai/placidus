package mirakurun

import "time"

// Service describes one Mirakurun broadcast service.
type Service struct {
	ID                 uint64   `json:"id"`
	ServiceID          int      `json:"serviceId"`
	NetworkID          int      `json:"networkId"`
	Name               string   `json:"name"`
	Type               int      `json:"type"`
	LogoID             int      `json:"logoId,omitempty"`
	HasLogoData        bool     `json:"hasLogoData,omitempty"`
	RemoteControlKeyID int      `json:"remoteControlKeyId,omitempty"`
	Channel            *Channel `json:"channel,omitempty"`
}

// Channel is the physical channel associated with a service.
type Channel struct {
	Type    string `json:"type"`
	Channel string `json:"channel"`
	Name    string `json:"name,omitempty"`
}

// Program describes one Mirakurun EPG event.
type Program struct {
	ID          uint64            `json:"id"`
	EventID     int               `json:"eventId"`
	ServiceID   int               `json:"serviceId"`
	NetworkID   int               `json:"networkId"`
	StartAt     int64             `json:"startAt"`
	Duration    int64             `json:"duration"`
	IsFree      bool              `json:"isFree"`
	Name        string            `json:"name,omitempty"`
	Description string            `json:"description,omitempty"`
	Extended    map[string]string `json:"extended,omitempty"`
}

// StartTime converts Mirakurun's millisecond timestamp to time.Time.
func (p Program) StartTime() time.Time {
	return time.UnixMilli(p.StartAt)
}

// EndTime returns the exclusive end of a program.
func (p Program) EndTime() time.Time {
	return p.StartTime().Add(time.Duration(p.Duration) * time.Millisecond)
}

// Tuner reports the current state of a Mirakurun tuner.
type Tuner struct {
	Index       int         `json:"index"`
	Name        string      `json:"name"`
	Types       []string    `json:"types"`
	IsAvailable bool        `json:"isAvailable"`
	IsRemote    bool        `json:"isRemote"`
	IsFree      bool        `json:"isFree"`
	Users       []TunerUser `json:"users"`
}

// TunerUser describes a consumer currently attached to a tuner.
type TunerUser struct {
	ID       string `json:"id"`
	Priority int    `json:"priority"`
}
