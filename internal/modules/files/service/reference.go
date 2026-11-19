package service

import "gorm.io/gorm"

// SetReferenceCheck binds the known business owners at bootstrap, before serving requests.
var referenceCheck func(*gorm.DB, int64) error

func SetReferenceCheck(check func(*gorm.DB, int64) error) { referenceCheck = check }
