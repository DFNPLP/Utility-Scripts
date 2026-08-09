package persistence

import "time"

type HashData struct {
	Id      uint64
	Hash    []byte
	Path    string
	Name    string
	NewName string
}

type Tag struct {
	Id       uint64
	Name     string
	Comment  string
	ActionId uint64
}

type Action struct {
	Id              uint64
	Type            string
	FileDestination string
}

type Plan struct {
	Id               uint64
	CreatedTimestamp time.Time
}

type PlanStep struct {
	Id                uint64
	PlanId            uint64
	HashRecordId      uint64
	ActionId          uint64
	ExecutedTimestamp time.Time
	CreatedTimestamp  time.Time
}
