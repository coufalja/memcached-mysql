package memcached

import "errors"

const (
	StatusEnd         = "END\r\n"
	StatusError       = "ERROR\r\n"
	StatusServerError = "SERVER_ERROR\r\n"
	StatusClientError = "CLIENT_ERROR %s\r\n"
	StatusStored      = "STORED\r\n"
	StatusNotStored   = "NOT_STORED\r\n"
	StatusExists      = "EXISTS\r\n"
	StatusNotFound    = "NOT_FOUND\r\n"
	StatusDeleted     = "DELETED\r\n"
	StatusTouched     = "TOUCHED\r\n"
	StatusOK          = "OK\r\n"
	StatusVersion     = "VERSION %s\r\n"
	StatusValue       = "VALUE %s %d %d\r\n"
	StatusStat        = "STAT %s %s\r\n"
)

var (
	// ClientError is an error caused by an invalid command from the client.
	ClientError = errors.New(StatusClientError)
	// NotFound is returned when the key was not found.
	NotFound = errors.New(StatusNotFound)
	// ServerError is an error occurred servicing this request.
	ServerError = errors.New(StatusServerError)
	// Error is a generic error.
	Error = errors.New(StatusError)
)
