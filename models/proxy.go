package models

import "time"

type Proxy struct {
	IP         string    `json:"ip" bson:"ip"`
	Status     string    `json:"status" bson:"status"`
	Message    string    `json:"message" bson:"message"`
	ErrorCount int       `json:"error_count" bson:"error_count"`
	UpdatedAt  time.Time `json:"updated_at" bson:"updated_at"`
}
